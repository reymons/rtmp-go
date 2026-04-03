package rtmp

import "io"

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

type Packet struct {
	Channel   uint32
	Stream    uint32
	Type      uint8
	Timestamp uint32
	Length    uint32
	DataRaw   []byte
	Data      io.Reader
}

func (pack *Packet) discard() error {
	_, err := io.Copy(io.Discard, pack.Data)
	return err
}
