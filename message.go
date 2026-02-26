package rtmp

type Message interface{}

type BasicMessage interface {
	FromPacket(pack *Packet)
}

type VideoMessage struct {
	Timestamp uint32
	Data      []byte
}

func (m *VideoMessage) FromPacket(pack *Packet) {
	m.Timestamp = pack.Timestamp
	m.Data = pack.Data
}

type AudioMessage struct {
	Timestamp uint32
	Data      []byte
}

func (m *AudioMessage) FromPacket(pack *Packet) {
	m.Timestamp = pack.Timestamp
	m.Data = pack.Data
}
