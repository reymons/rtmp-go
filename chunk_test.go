package rtmp

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestChunk_EncodesChunkStreamID(t *testing.T) {
	type testCase struct {
		streamID    uint32
		expectError bool
	}

	cases := []testCase{
		{streamID: 0, expectError: true},
		{streamID: 1, expectError: true},
		{streamID: 65600, expectError: true},
		{streamID: 71020, expectError: true},
		{streamID: 2},
		{streamID: 63},
		{streamID: 64},
		{streamID: 255},
		{streamID: 300},
		{streamID: 319},
		{streamID: 320},
		{streamID: 1017},
		{streamID: 51770},
		{streamID: 65599},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("stream id %d", tt.streamID), func(t *testing.T) {
			t.Parallel()

			chunk := Chunk{StreamID: tt.streamID}
			data, err := chunk.Encode()

			if err != nil {
				if !tt.expectError {
					t.Fatalf("test case: %+v, error: %v", tt, err)
				}

				if !errors.Is(err, ErrInvalidChunkStreamID) {
					t.Errorf("invalid error for stream ID (%d): expected %v, got %v",
						tt.streamID,
						ErrInvalidChunkStreamID,
						err)
				}

				return
			}

			id := uint32(data[0] & 0x3F)

			if tt.streamID > 63 {
				lowerByte := id
				expectedLowerByte := uint32(0)

				if tt.streamID > 319 {
					expectedLowerByte = 1
					id = uint32(data[2])*256 + uint32(data[1]) + 64
				} else {
					id = uint32(data[1]) + 64
				}

				if lowerByte != expectedLowerByte {
					t.Errorf("invalid stream id (%d) lower byte: expected %d, got %d", tt.streamID, expectedLowerByte, lowerByte)
				}

			}

			if id != tt.streamID {
				t.Errorf("invalid stream id: expected %d, got %d", tt.streamID, id)
			}
		})
	}
}

func TestChunk_EncodesChunkType(t *testing.T) {
	var chunk Chunk
	chunk.StreamID = 2

	for i := range uint8(100) {
		chunk.Type = i
		data, err := chunk.Encode()

		if err != nil {
			if i > maxChunkType {
				if !errors.Is(err, ErrInvalidChunkType) {
					t.Errorf("invalid error for chunk type %d: expected %v, got %v",
						i, ErrInvalidChunkType, err)
				}

				continue
			}

			t.Fatalf("chunk type %d: %v", i, err)
		}

		typ := (data[0] & 0xC0) >> 6
		if typ != i {
			t.Errorf("invalid chunk type: expected %d, got %d", i, typ)
		}
	}
}

func TestChunk_EncodesMessageHeader(t *testing.T) {
	type testCase struct {
		name         string
		chunk        *Chunk
		expectedData []byte
	}

	cases := []testCase{
		{
			name: "encodes type 0 message header",
			chunk: &Chunk{
				Type:            0,
				MessageLength:   128,
				MessageType:     20,
				MessageStreamID: 17,
				Timestamp:       28,
			},
			expectedData: []byte{0, 0, 0x1C, 0, 0, 0x80, 0x14, 0x11, 0, 0, 0},
		},
		{
			name: "encodes type 1 message header",
			chunk: &Chunk{
				Type:          1,
				MessageLength: 2000,
				MessageType:   3,
				Timestamp:     10017,
			},
			expectedData: []byte{0, 0x27, 0x21, 0, 0x07, 0xD0, 0x03},
		},
		{
			name: "encodes type 2 message header",
			chunk: &Chunk{
				Type:          2,
				MessageLength: 54017,
				MessageType:   241,
				Timestamp:     0xA0FFFF,
			},
			expectedData: []byte{0xA0, 0xFF, 0xFF},
		},
		{
			name: "encodes type 3 message header",
			chunk: &Chunk{
				Type:          3,
				MessageLength: 2000,
				MessageType:   3,
				Timestamp:     10017,
			},
			expectedData: []byte{},
		},
		{
			name: "encodes type 2 message header with extended timestamp that just overflowed",
			chunk: &Chunk{
				Type:          2,
				MessageLength: 2000,
				MessageType:   3,
				Timestamp:     0xFFFFFF,
			},
			expectedData: []byte{0xFF, 0xFF, 0xFF, 0, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "encodes type 2 message header with extended timestamp",
			chunk: &Chunk{
				Type:          2,
				MessageLength: 2000,
				MessageType:   3,
				Timestamp:     0x1CA0B4FF,
			},
			expectedData: []byte{0xFF, 0xFF, 0xFF, 0x1C, 0xA0, 0xB4, 0xFF},
		},
		{
			name: "encodes type 0 message header with extended timestamp",
			chunk: &Chunk{
				Type:            0,
				MessageLength:   128,
				MessageType:     20,
				MessageStreamID: 17,
				Timestamp:       0x7FFF3CCA,
			},
			expectedData: []byte{0xFF, 0xFF, 0xFF, 0, 0, 0x80, 0x14, 0x11, 0, 0, 0, 0x7F, 0xFF, 0x3C, 0xCA},
		},
	}

	for i, tt := range cases {
		tt.chunk.StreamID = uint32(i + 2)
		tt.chunk.Data = make([]byte, i+2)

		if _, err := rand.Read(tt.chunk.Data); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := tt.chunk.Encode()
			if err != nil {
				t.Fatalf("chunk.Encode: stream ID %d, %v", tt.chunk.StreamID, err)
			}

			data = data[tt.chunk.getMesgHdrOft():]
			total := make([]byte, len(tt.expectedData)+len(tt.chunk.Data))
			copy(total, tt.expectedData)
			copy(total[len(tt.expectedData):], tt.chunk.Data)

			if !bytes.Equal(total, data) {
				t.Errorf("invalid total encoded data: chunk stream ID %d, expected %x, got %x",
					tt.chunk.StreamID, total, data)
			}
		})
	}
}

func TestChunk_DecodesHeader(t *testing.T) {
	type testCase struct {
		name  string
		chunk *Chunk
		hdr   []byte
	}

	cases := []testCase{
		{
			name: "decodes chunk with type 0",
			chunk: &Chunk{
				Type:            0,
				StreamID:        60,
				Timestamp:       0x00FFFF,
				MessageStreamID: 2,
				MessageLength:   1000,
				MessageType:     20,
			},
			hdr: []byte{0x3C, 0, 0xFF, 0xFF, 0, 0x03, 0xE8, 0x14, 0x02, 0, 0, 0},
		},
		{
			name: "decodes chunk with type 0 and extended timestamp",
			chunk: &Chunk{
				Type:            0,
				StreamID:        60,
				Timestamp:       0xACFFFFFF,
				MessageStreamID: 2,
				MessageLength:   1000,
				MessageType:     20,
			},
			hdr: []byte{0x3C, 0xFF, 0xFF, 0xFF, 0, 0x03, 0xE8, 0x14, 0x02, 0, 0, 0, 0xAC, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 1",
			chunk: &Chunk{
				Type:          1,
				StreamID:      28,
				Timestamp:     0x003CFFA2,
				MessageLength: 4017,
				MessageType:   138,
			},
			hdr: []byte{0x5C, 0x3C, 0xFF, 0xA2, 0, 0x0F, 0xB1, 0x8A},
		},
		{
			name: "decodes chunk with type 1 and extended timestamp",
			chunk: &Chunk{
				Type:          1,
				StreamID:      28,
				Timestamp:     0x3A3CFFFF,
				MessageLength: 4017,
				MessageType:   138,
			},
			hdr: []byte{0x5C, 0xFF, 0xFF, 0xFF, 0, 0x0F, 0xB1, 0x8A, 0x3A, 0x3C, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 2",
			chunk: &Chunk{
				Type:      2,
				StreamID:  60,
				Timestamp: 0x00FFFF,
			},
			hdr: []byte{0xBC, 0, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 2 and extended timestamp",
			chunk: &Chunk{
				Type:      2,
				StreamID:  60,
				Timestamp: 0x1A1AFFFF,
			},
			hdr: []byte{0xBC, 0xFF, 0xFF, 0xFF, 0x1A, 0x1A, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 3",
			chunk: &Chunk{
				Type:     3,
				StreamID: 60,
			},
			hdr: []byte{0xFC},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chunk := &Chunk{}

			if err := chunk.DecodeHeader(bytes.NewReader(tt.hdr)); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.chunk, chunk); diff != "" {
				t.Errorf("invalid decoded chunk: %s", diff)
			}
		})
	}
}
