package rtmp

import (
	"fmt"
	"os/exec"
	"testing"
)

const (
	ErrorPolicyDefault uint8 = iota
	ErrorPolicyFailTest
)

type RTMPServerOptions struct {
	Addr        string
	Stream      uint32
	ErrorPolicy uint8
	OnConnect   func(conn *Conn, mesg *ConnectMessage) error
	OnPublish   func(conn *Conn, mesg *PublishStreamMessage) error
	OnMessage   func(conn *Conn, mesg Message) error
}

func defaultConnectHandler(conn *Conn, mesg *ConnectMessage) error {
	return nil
}

func defaultPublishHandler(conn *Conn, mesg *PublishStreamMessage) error {
	return nil
}

func defaultMessageHandler(conn *Conn, mesg Message) error {
	return nil
}

type RTMPServer struct {
	opts            RTMPServerOptions
	ln              Listener
	manuallyStopped bool
}

func NewRTMPServer(userOpts *RTMPServerOptions) *RTMPServer {
	srv := &RTMPServer{opts: *userOpts}

	if srv.opts.OnConnect == nil {
		srv.opts.OnConnect = defaultConnectHandler
	}
	if srv.opts.OnPublish == nil {
		srv.opts.OnPublish = defaultPublishHandler
	}
	if srv.opts.OnMessage == nil {
		srv.opts.OnMessage = defaultMessageHandler
	}
	return srv
}

func (s *RTMPServer) onConn(conn *Conn, t *testing.T) {
	defer conn.Close()

	stream := s.opts.Stream
	if stream == 0 {
		var err error
		stream, err = conn.AcceptStream(&AcceptStreamOptions{
			OnConnect: func(mesg *ConnectMessage) error {
				return s.opts.OnConnect(conn, mesg)
			},
			OnPublish: func(mesg *PublishStreamMessage) error {
				return s.opts.OnPublish(conn, mesg)
			},
		})
		if err != nil {
			t.Fatalf("accept stream: %v", err)
			return
		}
	}

	for {
		mesg, err := conn.ReadStreamMessage(stream)
		if err != nil {
			if err == ErrUnsupportedMessage {
				continue
			}
			if err != ErrConnClosed {
				t.Fatalf("read stream message: %v", err)
			}
			return
		}

		if err := s.opts.OnMessage(conn, mesg); err != nil {
			if s.opts.ErrorPolicy == ErrorPolicyFailTest {
				t.Fatalf("onMessage: %v", err)
			}
		}
	}
}

func (s *RTMPServer) Start(t *testing.T) error {
	defer func() {
		s.ln = nil
		s.manuallyStopped = false
	}()

	ln, err := Listen(s.opts.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.ln = ln

	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.manuallyStopped {
				return nil
			}
			return fmt.Errorf("accept connection: %w", err)
		}

		go s.onConn(conn, t)
	}
}

func (s *RTMPServer) Stop() error {
	if s.ln == nil {
		return fmt.Errorf("server is not running")
	}
	s.manuallyStopped = true
	return s.ln.Close()
}

type RTMPStreamer interface {
	Stream() error
}

type FFmpegStreamer struct {
	opts FFmpegOptions
}

type FFmpegOptions struct {
	Addr     string
	Filepath string
	Secure   bool
	AppName  string
}

func NewFFmpegStreamer(opts *FFmpegOptions) *FFmpegStreamer {
	return &FFmpegStreamer{opts: *opts}
}

func (s *FFmpegStreamer) Stream() error {
	proto := "rtmp"
	if s.opts.Secure {
		proto = "rtmps"
	}

	return exec.Command("ffmpeg",
		"-re",
		"-i", s.opts.Filepath,
		"-c:v", "libx264", // h.264 encoding
		"-preset", "veryfast",
		"-f", "flv",
		fmt.Sprintf("%s://%s/%s", proto, s.opts.Addr, s.opts.AppName),
	).Run()
}
