package rtmp

import "io"

type Message interface{}

type BasicMessage interface{}

type VideoMessage struct {
	Timestamp uint32
	Length    uint32
	Data      io.Reader
}

type AudioMessage struct {
	Timestamp uint32
	Length    uint32
	Data      io.Reader
}
