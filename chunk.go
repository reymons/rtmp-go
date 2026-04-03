package rtmp

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	chunkLargest uint8 = iota
	chunkLarge
	chunkSmall
	chunkSmallest

	chunkTypeCount
)

const maxChunkHdrSize = 18 // base hdr + mesg hdr + ext timestamp
const maxPackTimestamp = 0x00FFFFFF

var (
	ErrInvalidChunkType    = errors.New("invalid chunk type")
	ErrInvalidChunkChannel = errors.New("invalid chunk channel")
)

var chunkTmpBuf = [maxChunkHdrSize]byte{}

type rtmpChunk struct {
	chunkType  uint8
	channel    uint32
	timestamp  uint32
	packType   uint8
	packStream uint32
	packLength uint32
	// Used for decoding as a temporary buffer
	buf [maxChunkHdrSize]byte
}

func (c *rtmpChunk) decode(r io.Reader) error {
	var chunkType, packType uint8
	var channel, timestamp, packStream, packLength uint32
	var buf = c.buf[:]

	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		return err
	}

	chunkType = (buf[0] & 0xC0) >> 6
	if chunkType >= chunkTypeCount {
		return ErrInvalidChunkType
	}

	channel = uint32(buf[0]) & 0x3F
	if channel == 1 {
		if _, err := io.ReadFull(r, buf[:2]); err != nil {
			return err
		}
		channel = uint32(buf[1])*256 + uint32(buf[0]) + 64
	} else if channel == 0 {
		if _, err := io.ReadFull(r, buf[:1]); err != nil {
			return err
		}
		channel = uint32(buf[0]) + 64
	}

	switch chunkType {
	case chunkLargest:
		if _, err := io.ReadFull(r, buf[:11]); err != nil {
			return err
		}
		timestamp = decode3BytesBE(buf)
		packLength = decode3BytesBE(buf[3:])
		packType = buf[6]
		packStream = binary.LittleEndian.Uint32(buf[7:])
	case chunkLarge:
		if _, err := io.ReadFull(r, buf[:7]); err != nil {
			return err
		}
		timestamp = decode3BytesBE(buf)
		packLength = decode3BytesBE(buf[3:])
		packType = buf[6]
	case chunkSmall:
		if _, err := io.ReadFull(r, buf[:3]); err != nil {
			return err
		}
		timestamp = decode3BytesBE(buf)
	case chunkSmallest:
		// do nothing
	}

	// Handle extended timestamp
	if timestamp == maxPackTimestamp {
		if _, err := io.ReadFull(r, buf[:4]); err != nil {
			return err
		}
		timestamp = binary.BigEndian.Uint32(buf)
	}

	c.chunkType = chunkType
	c.channel = channel
	c.timestamp = timestamp
	c.packType = packType
	c.packStream = packStream
	c.packLength = packLength
	return nil
}

func (c *rtmpChunk) encode(buf []byte) (int, error) {
	if len(buf) < maxChunkHdrSize {
		return 0, errBufNoSpace
	}
	if c.chunkType >= chunkTypeCount {
		return 0, ErrInvalidChunkType
	}
	if c.channel < 2 || c.channel > 65599 {
		return 0, ErrInvalidChunkChannel
	}

	off := 1
	buf[0] = (c.chunkType << 6) | (uint8(c.channel) & 0x3F)

	if c.channel > 319 {
		buf[0] = (buf[0] & 0xC0) | 1
		buf[1] = byte((c.channel - 64) % 256)
		buf[2] = byte((c.channel - 64) / 256)
		off += 2
	} else if c.channel > 63 {
		buf[0] &= 0xC0
		buf[1] = byte(c.channel - 64)
		off += 1
	}

	hdr := buf[off:]
	switch c.chunkType {
	case chunkLargest:
		encode3BytesBE(hdr, c.timestamp)
		encode3BytesBE(hdr[3:], c.packLength)
		hdr[6] = c.packType
		binary.LittleEndian.PutUint32(hdr[7:], c.packStream)
		off += 11
	case chunkLarge:
		encode3BytesBE(hdr, c.timestamp)
		encode3BytesBE(hdr[3:], c.packLength)
		hdr[6] = c.packType
		off += 7
	case chunkSmall:
		encode3BytesBE(hdr, c.timestamp)
		off += 3
	case chunkSmallest:
		// do nothing
	}

	// Handle extended timestamp
	if c.chunkType < chunkSmallest && c.timestamp >= maxPackTimestamp {
		encode3BytesBE(hdr, maxPackTimestamp)
		binary.BigEndian.PutUint32(buf[off:], c.timestamp)
		off += 4
	}

	return off, nil
}
