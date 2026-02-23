package rtmp

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestHandshake_DoesHandshake(t *testing.T) {
	c1, c2 := net.Pipe()
	cliConn := newConn(c1)
	srvConn := newConn(c2)

	var wg sync.WaitGroup
	wg.Add(2)

	if err := c1.SetDeadline(time.Now().Add(time.Second * 5)); err != nil {
		t.Fatalf("set readline: %v", err)
	}
	if err := c2.SetDeadline(time.Now().Add(time.Second * 5)); err != nil {
		t.Fatalf("set readline: %v", err)
	}

	go func() {
		defer wg.Done()
		defer cliConn.Close()

		if err := handshakeServer(cliConn); err != nil {
			t.Fatalf("server handshake: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		defer srvConn.Close()

		if err := handshakeClient(srvConn); err != nil {
			t.Fatalf("client handshake: %v", err)
		}
	}()

	wg.Wait()
}
