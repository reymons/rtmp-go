package amf

import (
	"testing"
)

func newCodec(size int) (Encoder, Decoder, []byte) {
	data := make([]byte, size)
	enc := NewEncoder(data)
	dec := NewDecoder(data)
	return enc, dec, data
}

func TestDecoder_GetString(t *testing.T) {
	enc, dec, _ := newCodec(20)
	str := "hello world"

	if err := enc.PutString(str); err != nil {
		t.Fatal(err)
	}

	thisStr, err := dec.GetString()
	if err != nil {
		t.Fatal(err)
	}

	if thisStr != str {
		t.Errorf("wrong string: expected %s, got %s", str, thisStr)
	}
}

func TestDecoder_GetFloat64(t *testing.T) {
	enc, dec, _ := newCodec(20)
	num := 12030213.1231231

	if err := enc.PutFloat64(num); err != nil {
		t.Fatal(err)
	}

	thisNum, err := dec.GetFloat64()
	if err != nil {
		t.Fatal(err)
	}

	if thisNum != num {
		t.Errorf("wrong number: expected %f, got %f", num, thisNum)
	}
}

func TestDecoder_GetObject(t *testing.T) {
	enc, dec, _ := newCodec(100)

	obj := NewObject()
	obj.AddProp("name", "Danil")
	obj.AddProp("age", 25)

	enc.PutObject(obj)

	obj, err := dec.GetObject()
	if err != nil {
		t.Fatal(err)
	}

	if p, err := obj.GetProp("name"); err == nil {
		name := p.GetValue().GetString()
		if name != "Danil" {
			t.Errorf("invalid string: expected Danil, got %s", name)
		}
	} else {
		t.Fatal(err)
	}

	if p, err := obj.GetProp("age"); err == nil {
		age := p.GetValue().GetFloat64()
		if age != 25 {
			t.Errorf("invalid number: expected 25, got %f", age)
		}
	} else {
		t.Fatal(err)
	}
}
