package rtmp

import "net"

func Dial(addr string) (*Conn, error) {
	org, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	conn := newConn(org)

	if err := handshakeClient(conn); err != nil {
		return nil, err
	}

	return conn, err
}
