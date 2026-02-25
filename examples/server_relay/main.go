package main

import (
	"errors"
	"fmt"
	"runtime/debug"

	"rtmp"
)

func handleConnect(mesg *rtmp.ConnectMessage) error {
	fmt.Printf("Connect message: %s\n", mesg.AppName)
	return nil
}

func handlePublish(mesg *rtmp.PublishStreamMessage) error {
	fmt.Printf("Publish stream message: %v\n", mesg)
	return nil
}

func handle(conn *rtmp.Conn) error {
	stream, err := conn.AcceptStream(&rtmp.AcceptStreamOptions{
		OnConnect: handleConnect,
		OnPublish: handlePublish,
	})
	if err != nil {
		return fmt.Errorf("accept stream: %w", err)
	}

	fmt.Printf("New stream: %d\n", stream)

	for {
		mesg, err := conn.ReadStreamMessage(stream)
		if err != nil {
			if errors.Is(err, rtmp.ErrInvalidPacket) {
				continue
			}
			return fmt.Errorf("read message: %w", err)
		}

		switch m := mesg.(type) {
		case *rtmp.VideoMessage:
			fmt.Printf("Video message: %d, %d\n", m.Timestamp, len(m.Data))
		case *rtmp.AudioMessage:
			fmt.Printf("Audio message: %d, %d\n", m.Timestamp, len(m.Data))
		case *rtmp.CloseStreamMessage:
			fmt.Printf("Close stream: stream %d, closed stream %d\n", stream, m.Stream)
			return nil
		}
	}

	return nil
}

func handleConn(conn *rtmp.Conn) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
			fmt.Println("Stack trace:")
			fmt.Println(string(debug.Stack()))
		}
	}()

	if err := handle(conn); err != nil {
		fmt.Printf("handle error: %v\n", err)
	}
}

func main() {
	ln, err := rtmp.Listen("localhost:1935")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("accept connection: %v\n", err)
			continue
		}

		go handleConn(conn)
	}
}
