package rtmp

import (
	"fmt"
	"io"
)

type chunkChannel struct {
	id             uint32
	timestamp      uint32
	timestampDelta uint32
	packStream     uint32
	packLength     uint32
	packType       uint8
	totalBytesLeft uint32
	chunkBytesLeft uint32
	conn           *Conn
}

func (c *chunkChannel) HandleChunk(chunk *rtmpChunk) {
	switch chunk.chunkType {
	case chunkLargest:
		c.timestamp = chunk.timestamp
		c.timestampDelta = 0
		c.packStream = chunk.packStream
		c.packLength = chunk.packLength
		c.packType = chunk.packType
		c.totalBytesLeft = chunk.packLength
	case chunkLarge:
		c.timestampDelta = chunk.timestamp
		c.timestamp += c.timestampDelta
		c.packLength = chunk.packLength
		c.packType = chunk.packType
		c.totalBytesLeft = chunk.packLength
	case chunkSmall:
		c.timestampDelta = chunk.timestamp
		c.timestamp += c.timestampDelta
	case chunkSmallest:
		// do nothing
	}

	left := c.conn.recvChunkSize
	if left > c.totalBytesLeft {
		left = c.totalBytesLeft
	}
	c.chunkBytesLeft = left
}

// Reads the message payload by chunks
// Returns io.EOF if the whole message payload has been read
func (c *chunkChannel) Read(data []byte) (int, error) {
	if c.chunkBytesLeft == 0 {
		if c.totalBytesLeft == c.packLength {
			return 0, io.EOF
		}
		channel, err := c.conn.readChunk()
		if err != nil {
			c.conn.Close()
			return 0, fmt.Errorf("read chunk: %w", err)
		}
		if channel.id != c.id {
			c.conn.Close()
			return 0, fmt.Errorf("got a channel of different ID than the currently read channel's")
		}
	}

	end := uint32(len(data))
	if end > c.chunkBytesLeft {
		end = c.chunkBytesLeft
	}

	n, err := c.conn.reader.Read(data[:end])
	if err != nil {
		return 0, fmt.Errorf("read chunk payload: %w", err)
	}
	if n > 0 {
		c.chunkBytesLeft -= uint32(n)
		c.totalBytesLeft -= uint32(n)

		if c.totalBytesLeft == 0 {
			c.totalBytesLeft = c.packLength
		}
	}
	return n, nil
}

type sendChunkChannel struct {
	id         uint32
	timestamp  uint32
	packStream uint32
	packLength uint32
	packType   uint8
}
