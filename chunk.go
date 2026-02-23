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

type Chunk struct {
	Type       uint8
	Channel    uint32
	Timestamp  uint32
	PackType   uint8
	PackStream uint32
	PackLength uint32
}

func (c *Chunk) Decode(r io.Reader) error {
	buf := make([]byte, maxChunkHdrSize)

	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		return err
	}

	c.Type = (buf[0] & 0xC0) >> 6
	if c.Type >= chunkTypeCount {
		return ErrInvalidChunkType
	}

	c.Channel = uint32(buf[0]) & 0x3F
	if c.Channel == 1 {
		if _, err := io.ReadFull(r, buf[:2]); err != nil {
			return err
		}
		c.Channel = uint32(buf[1])*256 + uint32(buf[0]) + 64
	} else if c.Channel == 0 {
		if _, err := io.ReadFull(r, buf[:1]); err != nil {
			return err
		}
		c.Channel = uint32(buf[0]) + 64
	}

	switch c.Type {
	case chunkLargest:
		if _, err := io.ReadFull(r, buf[:11]); err != nil {
			return err
		}
		c.Timestamp = decode3BytesBE(buf)
		c.PackLength = decode3BytesBE(buf[3:])
		c.PackType = buf[6]
		c.PackStream = binary.LittleEndian.Uint32(buf[7:])
	case chunkLarge:
		if _, err := io.ReadFull(r, buf[:7]); err != nil {
			return err
		}
		c.Timestamp = decode3BytesBE(buf)
		c.PackLength = decode3BytesBE(buf[3:])
		c.PackType = buf[6]
	case chunkSmall:
		if _, err := io.ReadFull(r, buf[:3]); err != nil {
			return err
		}
		c.Timestamp = decode3BytesBE(buf)
	case chunkSmallest:
		// do nothing
	}

	// Handle extended timestamp
	if c.Timestamp == maxPackTimestamp {
		if _, err := io.ReadFull(r, buf[:4]); err != nil {
			return err
		}
		c.Timestamp = binary.BigEndian.Uint32(buf)
	}

	return nil
}

func (c *Chunk) Encode(buf []byte) (int, error) {
	if len(buf) < maxChunkHdrSize {
		return 0, errBufNoSpace
	}
	if c.Type >= chunkTypeCount {
		return 0, ErrInvalidChunkType
	}
	if c.Channel < 2 || c.Channel > 65599 {
		return 0, ErrInvalidChunkChannel
	}

	off := 1
	buf[0] = (c.Type << 6) | (uint8(c.Channel) & 0x3F)

	if c.Channel > 319 {
		buf[0] = (buf[0] & 0xC0) | 1
		buf[1] = byte((c.Channel - 64) % 256)
		buf[2] = byte((c.Channel - 64) / 256)
		off += 2
	} else if c.Channel > 63 {
		buf[0] &= 0xC0
		buf[1] = byte(c.Channel - 64)
		off += 1
	}

	hdr := buf[off:]
	switch c.Type {
	case chunkLargest:
		encode3BytesBE(hdr, c.Timestamp)
		encode3BytesBE(hdr[3:], c.PackLength)
		hdr[6] = c.PackType
		binary.LittleEndian.PutUint32(hdr[7:], c.PackStream)
		off += 11
	case chunkLarge:
		encode3BytesBE(hdr, c.Timestamp)
		encode3BytesBE(hdr[3:], c.PackLength)
		hdr[6] = c.PackType
		off += 7
	case chunkSmall:
		encode3BytesBE(hdr, c.Timestamp)
		off += 3
	case chunkSmallest:
		// do nothing
	}

	// Handle extended timestamp
	if c.Type < chunkSmallest && c.Timestamp >= maxPackTimestamp {
		encode3BytesBE(hdr, maxPackTimestamp)
		binary.BigEndian.PutUint32(buf[off:], c.Timestamp)
		off += 4
	}

	return off, nil
}
