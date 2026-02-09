package rtmp

import (
	"net"
)

type Conn struct {
	org   net.Conn
	epoch uint32
}

func newConn(org net.Conn) *Conn {
	return &Conn{org}
}
