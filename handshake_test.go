package rtmp

import (
	"net"
	"sync"
	"testing"
)

func TestHandshake_DoesHandshake(t *testing.T) {
	c1, c2 := net.Pipe()
	cliConn := newConn(c1)
	srvConn := newConn(c2)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		if err := handshakeServer(cliConn); err != nil {
			t.Fatalf("server handshake: %v", err)
		}
	}()

	go func() {
		defer wg.Done()

		if err := handshakeClient(srvConn); err != nil {
			t.Fatalf("client handshake: %v", err)
		}
	}()

	wg.Wait()
}
