package rtmp

const (
	PackSetChunkSize  uint8 = 1
	PackAbort               = 2
	PackAck                 = 3
	PackUserCtrl            = 4
	PackWinAckSize          = 5
	PackSetBandwidth        = 6
	PackAudio               = 8
	PackVideo               = 9
	PackCmdAMF0             = 20
	PackDataAMF0            = 18
	PackSharedObjAMF0       = 19
	PackAggregate           = 22
)

type Channel struct {
	ID             uint32
	Timestamp      uint32
	TimestampDelta uint32
	RecvBytes      uint32
	PackStream     uint32
	PackLength     uint32
	PackType       uint8
	RecvBuf        []byte
}

type Packet struct {
	Channel   uint32
	Stream    uint32
	Type      uint8
	Timestamp uint32
	Data      []byte
}

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
