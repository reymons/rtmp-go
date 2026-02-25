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
