# RTMP GO
RTMP implementation in Go

## Example of usage
You can check out an example [here](https://github.com/reymons/lively_backend/blob/master/transport/rtmp/rtmp.go)

## Caveats
- It is not a full implementation of RTMP spec but there is a basic functionality allowing one to work with live-streaming<br>
- The library does not work with AMF3 and there is no plans of adding it<br>
- There's also no convenient client-side API (you can only send raw packets) but I plan to add it in the future, someday...<br>

## Basic usage
The main idea is
- Start a server
- Accept a stream
- Receive messages on that stream

Here's an example of that
```golang
package main

import (
	"log"

	"github.com/reymons/rtmp-go"
)

func onConn(conn *rtmp.Conn) {
	defer conn.Close()

	// AcceptStream responds to `connect` and `publish` messages
	// You can do different validations at each step
	// If there's no error, you get an ID of the stream that was accepted
	stream, err := conn.AcceptStream(&rtmp.AcceptStreamOptions{
		OnConnect: func(mesg *rtmp.ConnectMessage, userData any) error {
			log.Printf("INFO: app name: %s", mesg.AppName)
			return nil
		},
		OnPublish: func(mesg *rtmp.PublishStreamMessage, userData any) error {
			return nil
		},
	})
	if err != nil {
		log.Printf("ERROR: accept stream: %v", err)
		return
	}
	log.Printf("INFO: accepted a stream %d", stream)

	for {
		mesg, err := conn.ReadStreamMessage(stream)
		if err != nil {
			if err == rtmp.ErrUnsupportedMessage {
				continue
			}
			log.Printf("ERROR: read stream message: %v", err)
			return
		}

		switch m := mesg.(type) {
		case *rtmp.VideoMessage:
			log.Printf("INFO: video message: timestamp %d", m.Timestamp)
		case *rtmp.AudioMessage:
			log.Printf("INFO: audio message: timestamp %d", m.Timestamp)
		case *rtmp.CloseStreamMessage:
			log.Printf("INFO: stream %d was closed", stream)
			return
		}
	}
}

func main() {
	addr := "localhost:1935"
	ln, err := rtmp.Listen(addr)
	if err != nil {
		log.Fatalf("ERROR: listen: %v", err)
	}
	log.Printf("INFO: running an RTMP server at %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("ERROR: accept connection: %v", err)
			continue
		}
		go onConn(conn)
	}
}

```

You can also use `ReadMessage` instead of `ReadStreamMessage` in case you want ot handle multiple streams
```golang
mesg, stream, err := conn.ReadMessage()

if stream == acceptedStream1 {
    // ...
} else if stream == acceptedStream2 {
    // ...
}
```
Or, if you wish, you can read raw packets using `ReadPacket`
```golang
var packet Packet
if err := conn.ReadPacket(&packet); err != nil {
    log.Printf("ERROR: read packet: %v", err)
    return
}

switch packet.Type {
case rtmp.PackVideo:
  // ...
case rtmp.PackAudio:
  // ...
}
```
Client-wise, there's nothing you can use at the moment except for `SendPacket`<br>
For example, send "Hello" as a video packet every 16ms for some hidden reason
```golang
var timestamp uint32
for {
    packet := rtmp.Packet{
        Channel: 2,
        Stream: 5,
        Type: rtmp.PackVideo,
        Timestamp: timestamp,
        Data: []byte("Hello"),
    }
    time.Sleep(time.Millisecond * 16)
    timestamp += 16
}

```

## Reference
- https://veovera.org/docs/legacy/rtmp-v1-0-spec.pdf
- https://en.wikipedia.org/wiki/Action_Message_Format
- https://ossrs.io/lts/en-us/assets/files/amf0_spec_121207-ac97fd4db9408706cd816b681ca3918c.pdf
