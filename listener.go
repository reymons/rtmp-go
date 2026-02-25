package rtmp

import "net"

type Listener interface {
	Accept() (*Conn, error)

	Close() error
}

func Listen(addr string) (Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &listener{l}, nil
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
