package rtmp

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func compareChunks(exp, got *rtmpChunk) error {
	if exp.chunkType != got.chunkType {
		return fmt.Errorf("type mismatch: expected %d, got %d", exp.chunkType, got.chunkType)
	}
	if exp.channel != got.channel {
		return fmt.Errorf("channel mismatch: expected %d, got %d", exp.channel, got.channel)
	}
	if exp.timestamp != got.timestamp {
		return fmt.Errorf("timestamp mismatch: expected %d, got %d", exp.timestamp, got.timestamp)
	}
	if exp.packType != got.packType {
		return fmt.Errorf("packet type mismatch: expected %d, got %d", exp.packType, got.packType)
	}
	if exp.packStream != got.packStream {
		return fmt.Errorf("packet stream mismatch: expected %d, got %d", exp.packStream, got.packStream)
	}
	if exp.packLength != got.packLength {
		return fmt.Errorf("packet length mismatch: expected %d, got %d", exp.packLength, got.packLength)
	}
	return nil
}

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

			chunk := rtmpChunk{channel: tt.channel}
			data := make([]byte, maxChunkHdrSize)

			if _, err := chunk.encode(data); err != nil {
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
	chunk := rtmpChunk{channel: 2}

	for i := range uint8(100) {
		chunk.chunkType = i
		data := make([]byte, maxChunkHdrSize)
		_, err := chunk.encode(data)

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
		chunk        *rtmpChunk
		expectedData []byte
	}

	cases := []testCase{
		{
			name: "encodes type 0 chunk",
			chunk: &rtmpChunk{
				chunkType:  chunkLargest,
				channel:    2,
				packLength: 128,
				packType:   20,
				packStream: 17,
				timestamp:  28,
			},
			expectedData: []byte{2, 0, 0, 0x1C, 0, 0, 0x80, 0x14, 0x11, 0, 0, 0},
		},
		{
			name: "encodes type 1 chunk",
			chunk: &rtmpChunk{
				chunkType:  chunkLarge,
				channel:    25,
				packLength: 2000,
				packType:   3,
				timestamp:  10017,
			},
			expectedData: []byte{0x59, 0, 0x27, 0x21, 0, 0x07, 0xD0, 0x03},
		},
		{
			name: "encodes type 2 chunk",
			chunk: &rtmpChunk{
				chunkType: chunkSmall,
				channel:   365,
				timestamp: 0xA0FFFF,
			},
			expectedData: []byte{0x81, 0x2D, 0x01, 0xA0, 0xFF, 0xFF},
		},
		{
			name: "encodes type 3 chunk",
			chunk: &rtmpChunk{
				chunkType: chunkSmallest,
				channel:   68,
			},
			expectedData: []byte{0xC0, 0x04},
		},
		{
			name: "encodes type 2 chunk with extended timestamp that just overflowed",
			chunk: &rtmpChunk{
				chunkType: chunkSmall,
				channel:   50801,
				timestamp: 0xFFFFFF,
			},
			expectedData: []byte{0x81, 0x31, 0xC6, 0xFF, 0xFF, 0xFF, 0, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "encodes type 2 chunk with extended timestamp",
			chunk: &rtmpChunk{
				chunkType: chunkSmall,
				channel:   3,
				timestamp: 0x1CA0B4FF,
			},
			expectedData: []byte{0x83, 0xFF, 0xFF, 0xFF, 0x1C, 0xA0, 0xB4, 0xFF},
		},
		{
			name: "encodes type 0 chunk with extended timestamp",
			chunk: &rtmpChunk{
				chunkType:  chunkLargest,
				channel:    10100,
				packLength: 128,
				packType:   20,
				packStream: 17,
				timestamp:  0x7FFF3CCA,
			},
			expectedData: []byte{0x01, 0x34, 0x27, 0xFF, 0xFF, 0xFF, 0, 0, 0x80, 0x14, 0x11, 0, 0, 0, 0x7F, 0xFF, 0x3C, 0xCA},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := make([]byte, maxChunkHdrSize)
			n, err := tt.chunk.encode(data)
			if err != nil {
				t.Fatalf("chunk.Encode: channel %d, %v", tt.chunk.channel, err)
			}
			data = data[:n]

			if !bytes.Equal(tt.expectedData, data) {
				t.Errorf("invalid chunk encoded data: channel %d, expected %x, got %x",
					tt.chunk.channel, tt.expectedData, data)
			}
		})
	}
}

func TestChunk_DecodesHeader(t *testing.T) {
	type testCase struct {
		name  string
		chunk *rtmpChunk
		hdr   []byte
	}

	cases := []testCase{
		{
			name: "decodes chunk with type 0",
			chunk: &rtmpChunk{
				chunkType:  0,
				channel:    60,
				timestamp:  0x00FFFF,
				packStream: 2,
				packLength: 1000,
				packType:   20,
			},
			hdr: []byte{0x3C, 0, 0xFF, 0xFF, 0, 0x03, 0xE8, 0x14, 0x02, 0, 0, 0},
		},
		{
			name: "decodes chunk with type 0 and extended timestamp",
			chunk: &rtmpChunk{
				chunkType:  0,
				channel:    60,
				timestamp:  0xACFFFFFF,
				packStream: 2,
				packLength: 1000,
				packType:   20,
			},
			hdr: []byte{0x3C, 0xFF, 0xFF, 0xFF, 0, 0x03, 0xE8, 0x14, 0x02, 0, 0, 0, 0xAC, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 1",
			chunk: &rtmpChunk{
				chunkType:  1,
				channel:    28,
				timestamp:  0x003CFFA2,
				packLength: 4017,
				packType:   138,
			},
			hdr: []byte{0x5C, 0x3C, 0xFF, 0xA2, 0, 0x0F, 0xB1, 0x8A},
		},
		{
			name: "decodes chunk with type 1 and extended timestamp",
			chunk: &rtmpChunk{
				chunkType:  1,
				channel:    28,
				timestamp:  0x3A3CFFFF,
				packLength: 4017,
				packType:   138,
			},
			hdr: []byte{0x5C, 0xFF, 0xFF, 0xFF, 0, 0x0F, 0xB1, 0x8A, 0x3A, 0x3C, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 2",
			chunk: &rtmpChunk{
				chunkType: 2,
				channel:   60,
				timestamp: 0x00FFFF,
			},
			hdr: []byte{0xBC, 0, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 2 and extended timestamp",
			chunk: &rtmpChunk{
				chunkType: 2,
				channel:   60,
				timestamp: 0x1A1AFFFF,
			},
			hdr: []byte{0xBC, 0xFF, 0xFF, 0xFF, 0x1A, 0x1A, 0xFF, 0xFF},
		},
		{
			name: "decodes chunk with type 3",
			chunk: &rtmpChunk{
				chunkType: 3,
				channel:   60,
			},
			hdr: []byte{0xFC},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chunk := &rtmpChunk{}

			if err := chunk.decode(bytes.NewReader(tt.hdr)); err != nil {
				t.Fatal(err)
			}

			if err := compareChunks(tt.chunk, chunk); err != nil {
				t.Errorf("compare chunks: %v", err)
			}
		})
	}
}
