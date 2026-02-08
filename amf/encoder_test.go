package amf

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func newEncoder(size int) (Encoder, []byte) {
	data := make([]byte, size)
	c := NewEncoder(data)
	return c, data
}

func testMarker(t *testing.T, marker uint8, data []byte, pos int) {
	if data[pos] != marker {
		t.Errorf("marker mismatch: expected %x, got %x", marker, data[pos])
	}
}

func TestEncoder_PutString(t *testing.T) {
	str := "Hello"
	c, data := newEncoder(len(str) + 4)

	if err := c.PutString(str); err != nil {
		t.Fatal(err)
	}

	testMarker(t, MarkerStr, data, 0)

	strLen := binary.BigEndian.Uint16(data[1:3])
	if strLen != uint16(len(str)) {
		t.Errorf("string length mismatch: expected %d, got %d", len(str), strLen)
	}

	encoded := string(data[3 : 3+len(str)])
	if encoded != str {
		t.Errorf("string mismatch: expected %s, got %s", str, encoded)
	}
}

func TestEncoder_PutFloat64(t *testing.T) {
	n := 40109102.51233
	c, data := newEncoder(9)

	if err := c.PutFloat64(n); err != nil {
		t.Fatal(err)
	}

	testMarker(t, MarkerNum, data, 0)

	num := math.Float64frombits(binary.BigEndian.Uint64(data[1:]))
	if num != n {
		t.Errorf("number mismatch: expected %f, got %f", n, num)
	}
}

func TestEncoder_PutUint(t *testing.T) {
	type testCase struct {
		num    uint64
		encode func(*Encoder) error
	}

	cases := []testCase{
		{
			11,
			func(c *Encoder) error {
				return c.PutUint8(11)
			},
		},
		{
			52017,
			func(c *Encoder) error {
				return c.PutUint32(52017)
			},
		},
		{
			128015,
			func(c *Encoder) error {
				return c.PutUint32(128015)
			},
		},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("number %d", tt.num), func(t *testing.T) {
			t.Parallel()

			c, data := newEncoder(9)

			if err := tt.encode(&c); err != nil {
				t.Fatal(err)
			}

			testMarker(t, MarkerNum, data, 0)

			num := uint64(math.Float64frombits(binary.BigEndian.Uint64(data[1:])))
			if num != tt.num {
				t.Errorf("number mismatch: expected %d, got %d", tt.num, num)
			}
		})
	}
}

func TestEncoder_StartObject(t *testing.T) {
	c, data := newEncoder(1)
	if err := c.StartObject(); err != nil {
		t.Fatal(err)
	}
	testMarker(t, MarkerObj, data, 0)
}

func TestEncoder_EndObject(t *testing.T) {
	c, data := newEncoder(3)
	data[0] = 0xfc // garbage
	data[1] = 0xc1 // garbage
	if err := c.EndObject(); err != nil {
		t.Fatal(err)
	}
	if data[0] != 0 || data[1] != 0 {
		t.Errorf("object end is incorrect: expected 0x0000, got 0x%x", data[0:2])
	}
	if data[2] != MarkerObjEnd {
		t.Errorf("object doesn't end with a correct marker: expected %x, got %x", MarkerObjEnd, data[2])
	}
}

func TestEncoder_AddObjectKey(t *testing.T) {
	c, data := newEncoder(50)
	key := "name"
	keyLen := uint16(len(key))

	if err := c.AddObjectKey(key); err != nil {
		t.Fatal(err)
	}

	dataKeyLen := binary.BigEndian.Uint16(data)
	dataKey := string(data[2 : 2+dataKeyLen])

	if dataKeyLen != keyLen {
		t.Errorf("incorrect key length: expected %d, got %d", keyLen, dataKeyLen)
	}

	if dataKey != key {
		t.Errorf("incorrect key: expected %s, got %s", key, dataKey)
	}
}
