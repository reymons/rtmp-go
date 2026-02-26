package rtmp

import (
	"sync"
	"testing"
	"time"
)

func TestConn_OutputsStreamedVideoToFile(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	wg.Add(2)

	appName := "something"
	onConnectCalledTimes := 0
	onPublishCalledTimes := 0

	srv := NewRTMPServer(&RTMPServerOptions{
		Addr: "localhost:1935",
		OnConnect: func(_ *Conn, mesg *ConnectMessage) error {
			if mesg.AppName != appName {
				t.Fatalf("invalid app name: expected %s, got %s", appName, mesg.AppName)
			}
			onConnectCalledTimes += 1
			return nil
		},
		OnPublish: func(_ *Conn, mesg *PublishStreamMessage) error {
			onPublishCalledTimes += 1
			return nil
		},
		OnMessage: func(conn *Conn, mesg Message) error {
			return nil
		},
	})

	go func() {
		defer wg.Done()

		if err := srv.Start(t); err != nil {
			t.Fatalf("run rtmp server: %v", err)
		}
	}()

	time.Sleep(time.Second)

	go func() {
		defer wg.Done()
		defer srv.Stop()
		streamer := NewFFmpegStreamer(&FFmpegOptions{
			Addr:     "localhost:1935",
			AppName:  appName,
			Filepath: "./testing/static/video.mp4",
			Secure:   false,
		})
		if err := streamer.Stream(); err != nil {
			t.Fatalf("ffmpeg: %v", err)
		}
	}()

	wg.Wait()

	if onConnectCalledTimes != 1 {
		t.Errorf("invalid connect call count: expected 1, got %d", onConnectCalledTimes)
	}
	if onPublishCalledTimes != 1 {
		t.Errorf("invalid publish call count: expected 1, got %d", onPublishCalledTimes)
	}
}

func TestConn_SendsMediaPacketsAtSameTime(t *testing.T) {
	t.Parallel()

	type mediaData struct {
		texts     []string
		textIndex int
		tDelta    uint32
		timestamp uint32
	}

	videoData := mediaData{
		texts: []string{
			"The coffee machine just made a noise that sounded suspiciously like a sigh",
			"I think I accidentally discovered a shortcut by getting completely lost",
			"Tomorrow feels like a good day to start something new",
			"The cat stared at the wall for five minutes — I’m slightly concerned",
			"I finally cleaned my desk, and now I can’t find anything",
		},
		tDelta: 16,
	}
	audioData := mediaData{
		texts: []string{
			"That moment when your code works on the first try… suspicious",
			"It’s amazing how fast time moves on a Friday afternoon",
			"I opened one tab and now there are seventeen",
			"The weather can’t decide what season it wants to be today",
			"Small progress is still progress — don’t forget that",
		},
		tDelta: 21,
	}
	stream := uint32(55)

	srv := NewRTMPServer(&RTMPServerOptions{
		Addr:   "localhost:1936",
		Stream: stream,
		OnMessage: func(conn *Conn, mesg Message) error {
			var text string
			var timestamp uint32
			var data *mediaData
			var packType uint8

			switch m := mesg.(type) {
			case *VideoMessage:
				text = string(m.Data)
				timestamp = m.Timestamp
				data = &videoData
				packType = PackVideo
			case *AudioMessage:
				text = string(m.Data)
				timestamp = m.Timestamp
				data = &audioData
				packType = PackAudio
			default:
				t.Fatalf("invalid message: %+v", mesg)
			}
			expectedText := data.texts[data.textIndex]
			if text != expectedText {
				t.Errorf("invalid text (pt %d): expected %s, got %s", packType, expectedText, text)
			}
			data.textIndex += 1
			if data.timestamp != timestamp {
				t.Errorf("invalid timestamp (pt %d): expected %d, got %d", packType, data.timestamp, timestamp)
			}
			data.timestamp += data.tDelta
			return nil
		},
	})

	go func() {
		defer srv.Stop()
		time.Sleep(time.Second)

		conn, err := Dial("localhost:1936")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}

		if err := conn.SetChunkSize(8); err != nil {
			t.Fatalf("set chunk size: %v", err)
		}

		sendData := func(wg *sync.WaitGroup, packType uint8, data *mediaData) {
			defer wg.Done()
			var timestamp uint32
			for i, text := range data.texts {
				pack := Packet{
					Channel:   4,
					Stream:    stream,
					Data:      []byte(text),
					Type:      packType,
					Timestamp: timestamp,
				}
				if err := conn.SendPacket(&pack); err != nil {
					t.Fatalf("send packet (pt %d, text %d): %v", packType, i, err)
				}
				time.Sleep(time.Millisecond * time.Duration(data.tDelta))
				timestamp += data.tDelta
			}
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go sendData(&wg, PackVideo, &videoData)
		go sendData(&wg, PackAudio, &audioData)
		wg.Wait()
	}()

	if err := srv.Start(t); err != nil {
		t.Fatalf("start server: %v", err)
	}

	if videoData.textIndex != len(videoData.texts) {
		t.Errorf("invalid onMessage call for video: expected %d, got %d", len(videoData.texts), videoData.textIndex)
	}
	if audioData.textIndex != len(audioData.texts) {
		t.Errorf("invalid onMessage call for audio: expected %d, got %d", len(audioData.texts), audioData.textIndex)
	}
}
