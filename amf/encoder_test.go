package amf

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func testMarker(t *testing.T, marker uint8, data []byte, pos int) {
	if data[pos] != marker {
		t.Errorf("marker mismatch: expected %x, got %x", marker, data[pos])
	}
}

func TestEncoder_PutString(t *testing.T) {
	enc := NewEncoder()
	str := "Hello"

	enc.PutString(str)
	testMarker(t, markerStr, enc.data, 0)

	strLen := binary.BigEndian.Uint16(enc.data[1:3])
	if strLen != uint16(len(str)) {
		t.Errorf("string length mismatch: expected %d, got %d", len(str), strLen)
	}

	encoded := string(enc.data[3 : 3+len(str)])
	if encoded != str {
		t.Errorf("string mismatch: expected %s, got %s", str, encoded)
	}
}

func TestEncoder_PutFloat64(t *testing.T) {
	enc := NewEncoder()
	n := 40109102.51233

	enc.PutFloat64(n)
	testMarker(t, markerNum, enc.data, 0)

	num := math.Float64frombits(binary.BigEndian.Uint64(enc.data[1:]))
	if num != n {
		t.Errorf("number mismatch: expected %f, got %f", n, num)
	}
}

func TestEncoder_PutUint(t *testing.T) {
	nums := []any{uint8(11), uint16(52017), uint32(128015)}

	for _, num := range nums {
		t.Run(fmt.Sprintf("number %d", num), func(t *testing.T) {
			t.Parallel()

			enc := NewEncoder()
			var expected uint64

			switch n := num.(type) {
			case uint8:
				expected = uint64(n)
				enc.PutUint8(n)
			case uint16:
				expected = uint64(n)
				enc.PutUint16(n)
			case uint32:
				expected = uint64(n)
				enc.PutUint32(n)
			default:
				t.Fatal("invalid integer type")
			}

			testMarker(t, markerNum, enc.data, 0)

			got := uint64(math.Float64frombits(binary.BigEndian.Uint64(enc.data[1:])))
			if expected != got {
				t.Errorf("number mismatch: expected %d, got %d", expected, got)
			}
		})
	}
}

func TestEncoder_AddObjectKey(t *testing.T) {
	enc := NewEncoder()
	key := "name"
	keyLen := uint16(len(key))

	if err := enc.AddObjectKey(key); err != nil {
		t.Fatal(err)
	}

	dataKeyLen := binary.BigEndian.Uint16(enc.data)
	dataKey := string(enc.data[2 : 2+dataKeyLen])

	if dataKeyLen != keyLen {
		t.Errorf("incorrect key length: expected %d, got %d", keyLen, dataKeyLen)
	}

	if dataKey != key {
		t.Errorf("incorrect key: expected %s, got %s", key, dataKey)
	}
}
