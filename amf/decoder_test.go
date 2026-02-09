package amf

import (
	"testing"
)

func TestDecoder_GetString(t *testing.T) {
	str := "hello world"

	enc := NewEncoder()
	enc.PutString(str)

	dec := NewDecoder(enc.Data())
	thisStr, err := dec.GetString()
	if err != nil {
		t.Fatal(err)
	}

	if thisStr != str {
		t.Errorf("wrong string: expected %s, got %s", str, thisStr)
	}
}

func TestDecoder_GetFloat64(t *testing.T) {
	num := 12030213.1231231

	enc := NewEncoder()
	enc.PutFloat64(num)

	dec := NewDecoder(enc.Data())
	thisNum, err := dec.GetFloat64()
	if err != nil {
		t.Fatal(err)
	}

	if thisNum != num {
		t.Errorf("wrong number: expected %f, got %f", num, thisNum)
	}
}

func TestDecoder_GetObject(t *testing.T) {
	obj := NewObject()
	obj.AddProp("name", "Danil")
	obj.AddProp("age", 25)

	enc := NewEncoder()
	enc.PutObject(obj)

	dec := NewDecoder(enc.Data())
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
