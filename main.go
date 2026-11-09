// Command DXT-decoder-bench times a JSON decoder on DXT book messages: it
// decodes every book in the sample many times over, on one pinned CPU, and
// reports allocations and the distribution of per-book decode times. Each
// decoder is made once, ahead of time, and always decodes into the same
// Book, as a hot path would use it. bench.sh runs each decoder in its own
// process.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"reflect"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/templexxx/tsc"
	"golang.org/x/sys/unix"
)

const (
	sample string = "sample.json"
	cpu    int    = 1
	rounds int    = 1000
	warmup int    = 100
	row    string = "%-8s %7s %10s %12s %11s %5s %10s %10s %10s %10s %10s %6s\n" // formatting hack
)

type Decoder interface {
	DecodeBook(book string, dst *Book) error
}

var decoders = map[string]func(copyStrings bool) Decoder{
	"gojson":  newGoJSONDecoder,
	"sonic":   newSonicDecoder,
	"jsonv2":  newJSONV2Decoder,
	"jsonv2h": newJSONV2HDecoder,
}

// This is probably right for getting an unsafe byte slice view over a string
func unsafeBytes(s string) []byte { return unsafe.Slice(unsafe.StringData(s), len(s)) }

func main() {
	name := flag.String("decoder", "", "the decoder: "+strings.Join(slices.Sorted(maps.Keys(decoders)), ", "))
	copyStrings := flag.Bool("copy", false, "make decoded strings copies, where the library would refer to the input")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile of the timed rounds to this file")
	flag.Parse()
	newDecoder, ok := decoders[*name]
	if !ok {
		fail("-decoder %q: want one of %s", *name, strings.Join(slices.Sorted(maps.Keys(decoders)), ", "))
	}

	runtime.LockOSThread()
	var set unix.CPUSet
	set.Set(cpu)
	if err := unix.SchedSetaffinity(0, &set); err != nil {
		fail("pin to CPU %d: %v", cpu, err)
	}
	if !tsc.Supported() {
		fail("no invariant TSC: timings would be the system clock's")
	}

	books, size, err := load(sample)
	if err != nil {
		fail("%v", err)
	}
	d := newDecoder(*copyStrings)
	var dst Book
	if err := check(d, &dst, books); err != nil {
		fail("%s: %v", *name, err)
	}

	fmt.Printf(row, "decoder", "books", "allocs", "allocs/book", "bytes/book", "GCs", "p50", "p90", "p99", "p99.9", "p99.99", "MB/s")

	times := make([]int64, 0, len(books)*rounds)
	r := run(d, &dst, books, warmup, rounds, &times, *cpuProfile)
	slices.Sort(times)
	at := func(q float64) string { return time.Duration(times[int(q*float64(len(times)-1))]).String() }
	// The mean, and so MB/s, leaves out the decodes above p99.99, as the
	// columns do.
	kept := times[:int(.9999*float64(len(times)-1))+1]
	var total int64
	for _, t := range kept {
		total += t
	}
	n := uint64(len(times))
	fmt.Printf(row, *name, fmt.Sprint(n), fmt.Sprint(r.allocs), fmt.Sprintf("%.1f", float64(r.allocs)/float64(n)),
		fmt.Sprint(r.bytes/n), fmt.Sprint(r.gcs), at(.5), at(.9), at(.99), at(.999), at(.9999),
		fmt.Sprintf("%.0f", float64(size)/float64(len(books))*float64(len(kept))/float64(total)*1e3))
}

type result struct {
	allocs, bytes uint64
	gcs           uint32
}

// run decodes every book warmup times untimed, then rounds times timed,
// each decode's time in ns going to times. With profile set, it writes a CPU
// profile of the timed rounds there.
func run(d Decoder, dst *Book, books []string, warmup, rounds int, times *[]int64, profile string) result {
	for range warmup {
		for _, b := range books {
			if err := d.DecodeBook(b, dst); err != nil {
				fail("%v", err)
			}
		}
	}
	runtime.GC()
	var before, after runtime.MemStats
	stop := startProfile(profile)
	runtime.ReadMemStats(&before)
	for range rounds {
		for _, b := range books {
			t0 := tsc.UnixNano()
			err := d.DecodeBook(b, dst)
			t1 := tsc.UnixNano()
			if err != nil {
				fail("%v", err)
			}
			*times = append(*times, t1-t0)
		}
	}
	runtime.ReadMemStats(&after)
	stop()
	return result{after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc, after.NumGC - before.NumGC}
}

// startProfile starts a CPU profile written to path, sampling at 1 kHz: the
// default 100 Hz gives a fast decoder's run only a few dozen samples. It
// returns what stops it. With no path it does nothing.
func startProfile(path string) (stop func()) {
	if path == "" {
		return func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		fail("cpu profile: %v", err)
	}
	// The rate can only be set before the profile starts, so StartCPUProfile
	// warns that it can't set it: the warning is harmless.
	runtime.SetCPUProfileRate(1000)
	if err := pprof.StartCPUProfile(f); err != nil {
		fail("cpu profile: %v", err)
	}
	return func() {
		pprof.StopCPUProfile()
		if err := f.Close(); err != nil {
			fail("cpu profile: %v", err)
		}
	}
}

func load(path string) ([]string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, 0, fmt.Errorf("%s: no books", path)
	}
	books := make([]string, len(raw))
	size := 0
	for i, r := range raw {
		books[i] = string(r)
		size += len(r)
	}
	return books, size, nil
}

func check(d Decoder, dst *Book, books []string) error {
	for i, b := range books {
		var want Book
		if err := json.Unmarshal([]byte(b), &want); err != nil {
			return fmt.Errorf("book %d: encoding/json: %w", i, err)
		}
		if err := d.DecodeBook(b, dst); err != nil {
			return fmt.Errorf("book %d: %w", i, err)
		}
		if !reflect.DeepEqual(*dst, want) {
			return fmt.Errorf("book %d: decoded differently from encoding/json", i)
		}
	}
	if err := d.DecodeBook(books[0]+` {}`, dst); err == nil {
		return errors.New("a book followed by more JSON was accepted")
	}
	return nil
}

// tscCost is the least time between two tsc reads: the floor under every timing.
func tscCost() int64 {
	least := int64(1 << 62)
	for range 10000 {
		t0 := tsc.UnixNano()
		least = min(least, tsc.UnixNano()-t0)
	}
	return least
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "decoder-bench: "+format+"\n", args...)
	os.Exit(1)
}
