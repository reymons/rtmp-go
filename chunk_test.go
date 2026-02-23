package rtmp

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestChunk_EncodesChunkChannel(t *testing.T) {
	type testCase struct {
		channel     uint32
		expectError bool
	}

	cases := []testCase{
		{channel: 0, expectError: true},
		{channel: 1, expectError: true},
		{channel: 65600, expectError: true},
		{channel: 71020, expectError: true},
		{channel: 2},
		{channel: 63},
		{channel: 64},
		{channel: 255},
		{channel: 300},
		{channel: 319},
		{channel: 320},
		{channel: 1017},
		{channel: 51770},
		{channel: 65599},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("stream id %d", tt.channel), func(t *testing.T) {
			t.Parallel()

			chunk := Chunk{Channel: tt.channel}
			data := make([]byte, maxChunkHdrSize)

			if _, err := chunk.Encode(data); err != nil {
				if !tt.expectError {
					t.Fatalf("test case: %+v, error: %v", tt, err)
				}

				if !errors.Is(err, ErrInvalidChunkChannel) {
					t.Errorf("invalid error for channel (%d): expected %v, got %v",
						tt.channel,
						ErrInvalidChunkChannel,
						err)
				}

				return
			}

			channel := uint32(data[0] & 0x3F)

			if tt.channel > 63 {
				lowerByte := channel
				expectedLowerByte := uint32(0)

				if tt.channel > 319 {
					expectedLowerByte = 1
					channel = uint32(data[2])*256 + uint32(data[1]) + 64
				} else {
					channel = uint32(data[1]) + 64
				}

				if lowerByte != expectedLowerByte {
					t.Errorf("invalid channel's (%d) lower byte: expected %d, got %d", tt.channel, expectedLowerByte, lowerByte)
				}

			}

			if channel != tt.channel {
				t.Errorf("invalchannel channel: expected %d, got %d", tt.channel, channel)
			}
		})
	}
}

func TestChunk_EncodesChunkType(t *testing.T) {
	chunk := Chunk{Channel: 2}

	for i := range uint8(100) {
		chunk.Type = i
		data := make([]byte, maxChunkHdrSize)
		_, err := chunk.Encode(data)

		if i >= chunkTypeCount {
			if err != nil {
				if !errors.Is(err, ErrInvalidChunkType) {
					t.Errorf("invalid error for chunk type %d: expected '%v', got '%v'",
						i, ErrInvalidChunkType, err)
				}
				continue
			}

			t.Fatalf("expected an invalid type error for chunk type %d", i)
		}

		typ := (data[0] & 0xC0) >> 6
		if typ != i {
			t.Errorf("invalid chunk type: expected %d, got %d", i, typ)
		}
	}
}

func TestChunk_EncodesFullChunk(t *testing.T) {
	type testCase struct {
		name         string
		chunk        *Chunk
		expectedData []byte
	}

	cases := []testCase{
		{
			name: "encodes type 0 chunk",
			chunk: &Chunk{
				Type:       chunkLargest,
				Channel:    2,
				PackLength: 128,
				PackType:   20,
				PackStream: 17,
				Timestamp:  28,
			},
			expectedData: []byte{2, 0, 0, 0x1C, 0, 0, 0x80, 0x14, 0x11, 0, 0, 0},
		},
		{
			name: "encodes type 1 chunk",
			chunk: &Chunk{
				Type:       chunkLarge,
				Channel:    25,
				PackLength: 2000,
				PackType:   3,
				Timestamp:  10017,
			},
			expectedData: []byte{0x59, 0, 0x27, 0x21, 0, 0x07, 0xD0, 0x03},
		},
		{
			name: "encodes type 2 chunk",
			chunk: &Chunk{
				Type:      chunkSmall,
				Channel:   365,
				Timestamp: 0xA0FFFF,
			},
			expectedData: []byte{0x81, 0x2D, 0x01, 0xA0, 0xFF, 0xFF},
		},
		{
			name: "encodes type 3 chunk",
			chunk: &Chunk{
				Type:    chunkSmallest,
				Channel: 68,
			},
			expectedData: []byte{0xC0, 0x04},
		},
		{
			name: "encodes type 2 chunk with extended timestamp that just overflowed",
			chunk: &Chunk{
				Type:      chunkSmall,
				Channel:   50801,
				Timestamp: 0xFFFFFF,
			},
			expectedData: []byte{0x81, 0x31, 0xC6, 0xFF, 0xFF, 0xFF, 0, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "encodes type 2 chunk with extended timestamp",
			chunk: &Chunk{
				Type:      chunkSmall,
				Channel:   3,
				Timestamp: 0x1CA0B4FF,
			},
			expectedData: []byte{0x83, 0xFF, 0xFF, 0xFF, 0x1C, 0xA0, 0xB4, 0xFF},
		},
		{
			name: "encodes type 0 chunk with extended timestamp",
			chunk: &Chunk{
				Type:       chunkLargest,
				Channel:    10100,
				PackLength: 128,
				PackType:   20,
				PackStream: 17,
				Timestamp:  0x7FFF3CCA,
			},
			expectedData: []byte{0x01, 0x34, 0x27, 0xFF, 0xFF, 0xFF, 0, 0, 0x80, 0x14, 0x11, 0, 0, 0, 0x7F, 0xFF, 0x3C, 0xCA},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := make([]byte, maxChunkHdrSize)
			n, err := tt.chunk.Encode(data)
			if err != nil {
				t.Fatalf("chunk.Encode: channel %d, %v", tt.chunk.Channel, err)
			}
			data = data[:n]

			if !bytes.Equal(tt.expectedData, data) {
				t.Errorf("invalid chunk encoded data: channel %d, expected %x, got %x",
					tt.chunk.Channel, tt.expectedData, data)
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
				Type:       0,
				Channel:    60,
				Timestamp:  0x00FFFF,
				PackStream: 2,
				PackLength: 1000,
				PackType:   20,
			},
			hdr: []byte{0x3C, 0, 0xFF, 0xFF, 0, 0x03, 0xE8, 0x14, 0x02, 0, 0, 0},
		},
		{
			name: "decodes chunk with type 0 and extended timestamp",
			chunk: &Chunk{
				Type:       0,
				Channel:    60,
				Timestamp:  0xACFFFFFF,
				PackStream: 2,
				PackLength: 1000,
				PackType:   20,
			},
			hdr: []byte{0x3C, 0xFF, 0xFF, 0xFF, 0, 0x03, 0xE8, 0x14, 0x02, 0, 0, 0, 0xAC, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 1",
			chunk: &Chunk{
				Type:       1,
				Channel:    28,
				Timestamp:  0x003CFFA2,
				PackLength: 4017,
				PackType:   138,
			},
			hdr: []byte{0x5C, 0x3C, 0xFF, 0xA2, 0, 0x0F, 0xB1, 0x8A},
		},
		{
			name: "decodes chunk with type 1 and extended timestamp",
			chunk: &Chunk{
				Type:       1,
				Channel:    28,
				Timestamp:  0x3A3CFFFF,
				PackLength: 4017,
				PackType:   138,
			},
			hdr: []byte{0x5C, 0xFF, 0xFF, 0xFF, 0, 0x0F, 0xB1, 0x8A, 0x3A, 0x3C, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 2",
			chunk: &Chunk{
				Type:      2,
				Channel:   60,
				Timestamp: 0x00FFFF,
			},
			hdr: []byte{0xBC, 0, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 2 and extended timestamp",
			chunk: &Chunk{
				Type:      2,
				Channel:   60,
				Timestamp: 0x1A1AFFFF,
			},
			hdr: []byte{0xBC, 0xFF, 0xFF, 0xFF, 0x1A, 0x1A, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 3",
			chunk: &Chunk{
				Type:    3,
				Channel: 60,
			},
			hdr: []byte{0xFC},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chunk := &Chunk{}

			if err := chunk.Decode(bytes.NewReader(tt.hdr)); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.chunk, chunk); diff != "" {
				t.Errorf("invalid decoded chunk: %s", diff)
			}
		})
	}
}
