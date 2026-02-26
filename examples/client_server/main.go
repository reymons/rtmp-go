package main

import (
	"errors"
	"fmt"
	"time"

	"rtmp"
)

func handleServerConn(conn *rtmp.Conn) {
	defer conn.Close()

	stream, err := conn.AcceptStream(&rtmp.AcceptStreamOptions{
		OnConnect: func(mesg *rtmp.ConnectMessage) error {
			return nil
		},
		OnPublish: func(mesg *rtmp.PublishStreamMessage) error {
			return nil
		},
	})
	if err != nil {
		fmt.Printf("accept stream error: %v\n", err)
		return
	}

	for {
		mesg, err := conn.ReadStreamMessage(stream)
		if err != nil {
			if errors.Is(err, rtmp.ErrInvalidPacket) {
				continue
			}
			fmt.Printf("read stream message error: %v\n", err)
			return
		}

		switch m := mesg.(type) {
		case *rtmp.VideoMessage:
			fmt.Printf("Video message: %d\n", m.Timestamp)
		case *rtmp.AudioMessage:
			fmt.Printf("Audio message: %d\n", m.Timestamp)
		case *rtmp.CloseStreamMessage:
			fmt.Printf("Stream closed. Closing the connection.\n")
			return
		}
	}
}

func runServer() {
	ln, err := rtmp.Listen("localhost:1935")
	if err != nil {
		fmt.Printf("listen error: %v\n", err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("accept error: %v\n", err)
			continue
		}

		go handleServerConn(conn)
	}
}

func runClient() {
	conn, err := rtmp.Dial("localhost:1935")
	if err != nil {
		fmt.Printf("dial error: %v\n", err)
		return
	}
	defer conn.Close()

	var trx uint32
	var mesg rtmp.CommandMessage

	mesg = &rtmp.ConnectMessage{
		AppName: "app/stream-key",
	}
	if err := conn.SendCommandMessage(mesg, trx, rtmp.ControlStream, 2); err != nil {
		fmt.Printf("send command message error: %v\n", err)
		return
	}
	trx += 1

	mesg = &rtmp.CreateStreamMessage{}
	if err := conn.SendCommandMessage(mesg, trx, rtmp.ControlStream, 2); err != nil {
		fmt.Printf("send command message error: %v\n", err)
		return
	}
	trx += 1

	mesg = &rtmp.PublishStreamMessage{
		PublishingName: "video",
		PublishingType: "record",
	}
	if err := conn.SendCommandMessage(mesg, trx, rtmp.ControlStream, 2); err != nil {
		fmt.Printf("send command message error: %v\n", err)
		return
	}
	trx += 1

	if err := conn.SetChunkSize(4); err != nil {
		panic(err)
	}

	go func() {
		var timestamp uint32
		for {
			pack := rtmp.Packet{
				Type:      rtmp.PackVideo,
				Channel:   2,
				Timestamp: timestamp,
				Data:      []byte("Hello world"),
				Stream:    1,
			}
			if err := conn.SendPacket(&pack); err != nil {
				fmt.Printf("send client video packet error: %v\n", err)
				return
			}
			time.Sleep(time.Millisecond * 16)
			timestamp += 16
		}
	}()

	go func() {
		var timestamp uint32
		for {
			pack := rtmp.Packet{
				Type:      rtmp.PackAudio,
				Channel:   2,
				Timestamp: timestamp,
				Data:      []byte("Hello world"),
				Stream:    1,
			}
			if err := conn.SendPacket(&pack); err != nil {
				fmt.Printf("send client audio packet error: %v\n", err)
				return
			}
			time.Sleep(time.Millisecond * 21)
			timestamp += 21
		}
	}()

	select {}
}

func main() {
	go runServer()
	time.Sleep(time.Second)
	go runClient()

	select {}
}
