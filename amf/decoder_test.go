package amf

import (
	"testing"
)

func TestDecoder_GetString(t *testing.T) {
	str := "hello world"

	enc := NewEncoder()
	enc.PutString(str)

	dec := NewDecoder()
	dec.SetData(enc.Data())

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

	dec := NewDecoder()
	dec.SetData(enc.Data())

	thisNum, err := dec.GetFloat64()
	if err != nil {
		t.Fatal(err)
	}

	if thisNum != num {
		t.Errorf("wrong number: expected %f, got %f", num, thisNum)
	}
}

func TestDecoder_GetObject(t *testing.T) {
	name := "Matt"
	age := 25

	obj := NewObject()
	obj.AddProp("name", name)
	obj.AddProp("age", age)

	enc := NewEncoder()
	enc.PutObject(&obj)

	dec := NewDecoder()
	dec.SetData(enc.Data())

	obj, err := dec.GetObject()
	if err != nil {
		t.Fatal(err)
	}

	thisName := obj.Prop("name").Value().String()
	thisAge := obj.Prop("age").Value().Uint32()

	if thisName != name {
		t.Errorf("invalid string: expected %s, got %s", name, thisName)
	}
	if thisAge != uint32(age) {
		t.Errorf("invalid number: expected %d, got %d", age, thisAge)
	}
}
