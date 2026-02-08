package rtmp

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrInvalidChunkType     = errors.New("invalid chunk type")
	ErrInvalidChunkStreamID = errors.New("invalid chunk stream ID")
)

const maxChunkType = 3
const maxMesgTimestamp = 0x00FFFFFF

type Chunk struct {
	StreamID        uint32
	Type            uint8
	Timestamp       uint32
	MessageStreamID uint32
	MessageLength   uint32
	MessageType     uint8
	Data            []byte
}

func (c *Chunk) getMesgHdrOft() int {
	if c.StreamID > 319 {
		return 3
	}
	if c.StreamID > 63 {
		return 2
	}
	return 1
}

func (c *Chunk) DecodeHeader(r io.Reader) error {
	var chunkType, mesgType uint8
	var chunkSID, mesgSID, mesgLen, timestamp uint32

	hdr := make([]byte, 11) // max mesg header size

	if _, err := io.ReadFull(r, hdr[:1]); err != nil {
		return err
	}

	chunkType = (hdr[0] & 0xC0) >> 6
	chunkSID = uint32(hdr[0]) & 0x3F

	if chunkType > maxChunkType {
		return ErrInvalidChunkType
	}

	if chunkSID < 2 {
		if chunkSID == 1 {
			if _, err := io.ReadFull(r, hdr[:2]); err != nil {
				return err
			}
			chunkSID = uint32(hdr[1])*256 + uint32(hdr[0]) + 64
		} else {
			if _, err := io.ReadFull(r, hdr[:1]); err != nil {
				return err
			}
			chunkSID = uint32(hdr[0]) + 64
		}
	}

	if chunkType < 3 {
		if _, err := io.ReadFull(r, hdr[:3]); err != nil {
			return err
		}

		timestamp = decode3BytesBE(hdr)

		if chunkType < 2 {
			if _, err := io.ReadFull(r, hdr[:4]); err != nil {
				return err
			}

			mesgLen = decode3BytesBE(hdr)
			mesgType = hdr[3]

			if chunkType == 0 {
				if _, err := io.ReadFull(r, hdr[:4]); err != nil {
					return err
				}
				mesgSID = binary.LittleEndian.Uint32(hdr)
			}
		}

		// Read extended timestamp
		if timestamp == maxMesgTimestamp {
			if _, err := io.ReadFull(r, hdr[:4]); err != nil {
				return err
			}
			timestamp = binary.BigEndian.Uint32(hdr)
		}
	}

	c.Type = chunkType
	c.StreamID = chunkSID
	c.Timestamp = timestamp
	c.MessageStreamID = mesgSID
	c.MessageLength = mesgLen
	c.MessageType = mesgType

	return nil
}

func (c *Chunk) Encode() (result []byte, err error) {
	var oft int
	var hdr []byte

	if c.Type > maxChunkType {
		err = ErrInvalidChunkType
		goto fail
	}

	if c.StreamID < 2 || c.StreamID > 65599 {
		err = ErrInvalidChunkStreamID
		goto fail
	}

	hdr = make([]byte, 3+11+4) // max base hdr size + max mesg hdr size + extended timestamp size
	hdr[0] = (c.Type & 0x03) << 6

	if c.StreamID < 64 {
		hdr[0] |= byte(c.StreamID & 0x3F)
		oft += 1
	} else {
		oft += 2

		if c.StreamID < 320 {
			hdr[1] = byte(c.StreamID - 64)
		} else {
			hdr[0] |= 1
			hdr[1] = byte((c.StreamID - 64) % 256)
			hdr[2] = byte((c.StreamID - 64) / 256)
			oft += 1
		}
	}

	if c.Type < 3 {
		var extTimestamp uint32

		timestamp := c.Timestamp
		if timestamp >= maxMesgTimestamp {
			extTimestamp = timestamp
			timestamp = maxMesgTimestamp
		}

		encode3BytesBE(hdr[oft:], timestamp)
		oft += 3

		if c.Type < 2 {
			encode3BytesBE(hdr[oft:], c.MessageLength)
			oft += 3

			hdr[oft] = c.MessageType
			oft += 1

			if c.Type == 0 {
				binary.LittleEndian.PutUint32(hdr[oft:], c.MessageStreamID)
				oft += 4
			}
		}

		if extTimestamp != 0 {
			binary.BigEndian.PutUint32(hdr[oft:], extTimestamp)
			oft += 4
		}
	}

	hdr = hdr[:oft]

	result = make([]byte, len(hdr)+len(c.Data))
	copy(result, hdr)
	copy(result[len(hdr):], c.Data)

	return result, nil

fail:
	return []byte{}, err
}

func logChunk(prfx string, c *Chunk) {
	Debugf(prfx+": CSID %d, CType %d, MSID %d, MType %d, MLen %d, TT %d\n",
		c.StreamID, c.Type, c.MessageStreamID, c.MessageType, c.MessageLength, c.Timestamp)
}

func fillChunkType0(c *Chunk, SID, mesgSID, mesgLen, timestamp uint32, mesgType uint8) {
	c.Type = 0
	c.StreamID = SID
	c.MessageStreamID = mesgSID
	c.MessageType = mesgType
	c.MessageLength = mesgLen
	c.Timestamp = timestamp
}

func fillChunkType1(c *Chunk, SID, mesgLen, timestamp uint32, mesgType uint8) {
	c.Type = 1
	c.StreamID = SID
	c.MessageType = mesgType
	c.MessageLength = mesgLen
	c.Timestamp = timestamp
}

func fillChunkType2(c *Chunk, SID, timestamp uint32) {
	c.Type = 2
	c.StreamID = SID
	c.Timestamp = timestamp
}

func fillChunkType3(c *Chunk, SID uint32) {
	c.Type = 3
	c.StreamID = SID
}
