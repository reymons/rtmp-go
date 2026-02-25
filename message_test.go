package rtmp

import (
	"bytes"
	"fmt"
	"testing"
)

func TestMessage_FromPacket(t *testing.T) {
	type testCase struct {
		name     string
		mesg     BasicMessage
		pack     *Packet
		validate func(mesg BasicMessage) error
	}

	cases := []testCase{
		{
			name: "video message",
			mesg: &VideoMessage{},
			pack: &Packet{Timestamp: 40, Data: []byte("hello")},
			validate: func(mesg BasicMessage) error {
				m := mesg.(*VideoMessage)
				timestamp := uint32(40)
				data := []byte("hello")

				if m.Timestamp != timestamp {
					return fmt.Errorf("invalid timestamp: expected %d, got %d", timestamp, m.Timestamp)
				}
				if !bytes.Equal(m.Data, data) {
					return fmt.Errorf("invalid data: expected %x, got %x\n", data, m.Data)
				}
				return nil
			},
		},
		{
			name: "audio message",
			mesg: &AudioMessage{},
			pack: &Packet{Timestamp: 10, Data: []byte("something")},
			validate: func(mesg BasicMessage) error {
				m := mesg.(*AudioMessage)
				timestamp := uint32(10)
				data := []byte("something")

				if m.Timestamp != timestamp {
					return fmt.Errorf("invalid timestamp: expected %d, got %d", timestamp, m.Timestamp)
				}
				if !bytes.Equal(m.Data, data) {
					return fmt.Errorf("invalid data: expected %x, got %x\n", data, m.Data)
				}
				return nil
			},
		},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("creates %s from packet", tt.name), func(t *testing.T) {
			t.Parallel()
			tt.mesg.FromPacket(tt.pack)

			if err := tt.validate(tt.mesg); err != nil {
				t.Errorf("validate message: %v", err)
			}
		})
	}
}
