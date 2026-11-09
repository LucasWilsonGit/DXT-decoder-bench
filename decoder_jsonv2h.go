package main

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"reflect"
	"unsafe"
)

// json/v2 with an UnmarshalJSONFrom for a side's levels, on hBook, a copy of
// Book only this decoder uses: on Book itself, encoding/json and plain json/v2
// would use it too. Strings are always copies, so the bool is ignored.
type jsonV2HDecoder struct{ opts jsonv2.Options }

type hBook struct {
	Type        string  `json:"type"`
	Instrument  string  `json:"instrument"`
	ID          string  `json:"id"`
	Seq         uint64  `json:"seq"`
	CleanSeq    uint64  `json:"cleanSeq"`
	ExchTime    int64   `json:"exchTime"`
	RecvTime    int64   `json:"recvTime"`
	DecodeTime  int64   `json:"decodeTime"`
	State       string  `json:"state"`
	Recovering  bool    `json:"recovering"`
	Tags        uint64  `json:"tags"`
	Bids        hLevels `json:"bids"`
	Asks        hLevels `json:"asks"`
	GwSeq       uint64  `json:"gwSeq"`
	PublishTime int64   `json:"publishTime"`
}

type hLevels [][2]string

func newJSONV2HDecoder(bool) Decoder {
	if err := sameLayout(reflect.TypeFor[Book](), reflect.TypeFor[hBook]()); err != nil {
		fail("jsonv2h: hBook isn't Book: %v", err)
	}
	return &jsonV2HDecoder{opts: jsonv2.JoinOptions(jsontext.AllowDuplicateNames(true), jsontext.AllowInvalidUTF8(true))}
}

func (d *jsonV2HDecoder) DecodeBook(book string, dst *Book) error {
	return jsonv2.Unmarshal(unsafeBytes(book), (*hBook)(unsafe.Pointer(dst)), d.opts)
}

var errLevel = errors.New(`a level must be ["price","qty"]`)

// UnmarshalJSONFrom decodes a side's levels, reusing its capacity: [] or
// null, or ["price","qty"] pairs.
func (s *hLevels) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	switch tok.Kind() {
	case 'n':
		*s = nil
		return nil
	case '[':
	default:
		return errLevel
	}
	ls := (*s)[:0]
	for dec.PeekKind() != ']' {
		if tok, err := dec.ReadToken(); err != nil {
			return err
		} else if tok.Kind() != '[' {
			return errLevel
		}
		var l [2]string
		for i := range l {
			v, err := dec.ReadValue()
			if err != nil {
				return err
			}
			if v.Kind() != '"' {
				return errLevel
			}
			if l[i], err = hString(v); err != nil {
				return err
			}
		}
		if tok, err := dec.ReadToken(); err != nil {
			return err
		} else if tok.Kind() != ']' {
			return errLevel
		}
		ls = append(ls, l)
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	*s = ls
	return nil
}

// hString is a copy of the string a JSON string value v holds.
func hString(v jsontext.Value) (string, error) {
	raw := v[1 : len(v)-1]
	if bytes.IndexByte(raw, '\\') < 0 {
		return string(raw), nil
	}
	u, err := jsontext.AppendUnquote(nil, v)
	return string(u), err
}

// sameLayout is whether structs a and b have the same fields, in the same
// places, with the same tags and the same underlying types, so a pointer to
// one may be used as a pointer to the other.
func sameLayout(a, b reflect.Type) error {
	if a.Size() != b.Size() || a.NumField() != b.NumField() {
		return errors.New("different sizes or numbers of fields")
	}
	for i := range a.NumField() {
		fa, fb := a.Field(i), b.Field(i)
		same := fa.Type == fb.Type || fa.Type.Kind() == reflect.Slice && fb.Type.Kind() == reflect.Slice && fa.Type.Elem() == fb.Type.Elem()
		if fa.Name != fb.Name || fa.Offset != fb.Offset || fa.Tag != fb.Tag || !same {
			return errors.New("field " + fa.Name + " differs")
		}
	}
	return nil
}
