package rtmp

import (
	"crypto/tls"
	"net"
)

type Listener interface {
	Accept() (*Conn, error)

	Close() error
}

func Listen(addr string) (Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &listener{ln}, nil
}

func ListenTLS(addr string, conf *tls.Config) (Listener, error) {
	ln, err := tls.Listen("tcp", addr, conf)
	if err != nil {
		return nil, err
	}
	return &listener{ln}, nil
}

type listener struct {
	listener net.Listener
}

func (l *listener) Accept() (*Conn, error) {
	org, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}

	conn := newConn(org)

	if err := handshakeServer(conn); err != nil {
		return nil, err
	}

	Debugf("New connection from %s\n", org.RemoteAddr())

	return conn, nil
}

func (l *listener) Close() error {
	return l.listener.Close()
}
