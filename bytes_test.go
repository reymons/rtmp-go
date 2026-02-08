package rtmp

import (
	"bytes"
	"fmt"
	"testing"
)

func TestBytes_EncodesBE(t *testing.T) {
	type testCase struct {
		num      uint32
		expected []byte
	}

	cases := []testCase{
		{0, []byte{0, 0, 0}},
		{17, []byte{0, 0, 0x11}},
		{0x00AABBCC, []byte{0xAA, 0xBB, 0xCC}},
		{0xFFA1B1C1, []byte{0xA1, 0xB1, 0xC1}},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("number %d", tt.num), func(t *testing.T) {
			t.Parallel()

			buf := make([]byte, 3)
			encode3BytesBE(buf, tt.num)

			if !bytes.Equal(buf, tt.expected) {
				t.Errorf("expected %x, got %x\n", tt.expected, buf)
			}
		})
	}
}

func TestBytes_DecodesBE(t *testing.T) {
	type testCase struct {
		expected uint32
		buf      []byte
		panics   bool
	}

	cases := []testCase{
		{0, []byte{0, 0, 0}, false},
		{17, []byte{0, 0, 0x11}, false},
		{0x00AABBCC, []byte{0xAA, 0xBB, 0xCC}, false},
		{0xBCBCBC, []byte{0xBC, 0xBC, 0xBC, 0xA1}, false},
		{0, []byte{}, true},
		{0, []byte{0}, true},
		{0, []byte{0, 0}, true},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("number %d", tt.expected), func(t *testing.T) {
			t.Parallel()

			if tt.panics {
				defer func() {
					if r := recover(); r == nil {
						t.Error("did not panic")
					}
				}()
			}

			got := decode3BytesBE(tt.buf)

			if got != tt.expected {
				t.Errorf("expected %d, got %d\n", tt.expected, got)
			}
		})
	}
}
