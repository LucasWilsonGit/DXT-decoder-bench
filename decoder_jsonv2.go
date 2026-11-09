package main

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
)

// json/v2 reads bytes from a buffer without copying, we'll disable some of
// the default checks that are usually on like valid UTF and duplicate names.
// json/v2 always copies strings, so for comparisons we'll run the wrapper with -copy fla
//
// json/v2 takes bytes, and never writes to them, so they may alias the
// string.

type jsonV2Decoder struct {
	buf  bytes.Buffer
	dec  *jsontext.Decoder
	opts jsonv2.Options
}

func newJSONV2Decoder(bool) Decoder {
	d := &jsonV2Decoder{opts: jsonv2.JoinOptions(jsontext.AllowDuplicateNames(true), jsontext.AllowInvalidUTF8(true))}
	d.dec = jsontext.NewDecoder(&d.buf, d.opts)
	return d
}

var errTrailing = errors.New("data after the book")

func (d *jsonV2Decoder) DecodeBook(book string, dst *Book) error {
	d.buf = *bytes.NewBuffer(unsafeBytes(book))
	d.dec.Reset(&d.buf, d.opts)
	if err := jsonv2.UnmarshalDecode(d.dec, dst); err != nil {
		return err
	}
	if d.dec.PeekKind() != 0 { // as Unmarshal does: nothing may follow the book
		return errTrailing
	}
	return nil
}
