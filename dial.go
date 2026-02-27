package rtmp

import (
	"crypto/tls"
	"net"
)

func onNetConn(org net.Conn) (*Conn, error) {
	conn := newConn(org)
	if err := handshakeClient(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

func Dial(addr string) (*Conn, error) {
	org, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return onNetConn(org)
}

func DialTLS(addr string, conf *tls.Config) (*Conn, error) {
	org, err := tls.Dial("tcp", addr, conf)
	if err != nil {
		return nil, err
	}
	return onNetConn(org)
}
