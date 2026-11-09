package main

import (
	"reflect"

	sonic "github.com/bytedance/sonic/decoder"
)

// Create a single decoder to be used for decoding all seen books, as good practise.
// Sonic does some cool things with strings referring into the input buffer.
// We can precompile the JIT and I'm on x86-64 hardware which sonic can handle well.

type sonicDecoder struct{ dec *sonic.Decoder }

func newSonicDecoder(copyStrings bool) Decoder {
	if err := sonic.Pretouch(reflect.TypeFor[Book]()); err != nil {
		fail("sonic: pretouch: %v", err)
	}
	d := &sonicDecoder{dec: sonic.NewDecoder("")}
	if copyStrings {
		d.dec.CopyString()
	}
	return d
}

func (d *sonicDecoder) DecodeBook(book string, dst *Book) error {
	d.dec.Reset(book)
	if err := d.dec.Decode(dst); err != nil {
		return err
	}
	return d.dec.CheckTrailings() // Chuck an error if there is trailing crap, this is to be consistent with json/v2
}
