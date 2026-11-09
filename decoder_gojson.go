package main

import gojson "github.com/goccy/go-json"

// go-json doesn't let us precompile the JIT but it is cached after the first decode
// so we just touch a decode when maker the decoder and we're golden.
// It does not refer to strings from the input buffer by default, but we can ask it to.

type goJSONDecoder struct{ opts []gojson.DecodeOptionFunc }

func newGoJSONDecoder(copyStrings bool) Decoder {
	d := &goJSONDecoder{opts: []gojson.DecodeOptionFunc{gojson.DecodeFieldPriorityFirstWin()}}
	if !copyStrings {
		d.opts = append(d.opts, gojson.DecodeNoCopyString())
	}
	var b Book
	_ = d.DecodeBook(`{}`, &b)
	return d
}

func (d *goJSONDecoder) DecodeBook(book string, dst *Book) error {
	return gojson.UnmarshalWithOption(unsafeBytes(book), dst, d.opts...)
}
