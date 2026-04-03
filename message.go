package rtmp

import "io"

type Message interface{}

type BasicMessage interface {
	FromPacket(pack *Packet)
}

type VideoMessage struct {
	Timestamp uint32
	Length    uint32
	Data      io.Reader
}

func (m *VideoMessage) FromPacket(pack *Packet) {
	m.Timestamp = pack.Timestamp
	m.Length = pack.Length
	m.Data = pack.Data
}

type AudioMessage struct {
	Timestamp uint32
	Length    uint32
	Data      io.Reader
}

func (m *AudioMessage) FromPacket(pack *Packet) {
	m.Timestamp = pack.Timestamp
	m.Length = pack.Length
	m.Data = pack.Data
}
