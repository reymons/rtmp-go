package rtmp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
)

var (
	errUnsupportedProtoVer = errors.New("unsupported protocol version")
	errSignatureMismatch   = errors.New("signature mismatch")
)

const sigSize = 1536
const allowedProtoVer uint8 = 3

func handshakeServer(conn *Conn) error {
	buf1 := make([]byte, sigSize+1)
	buf2 := make([]byte, sigSize+1)

	// Read client version and signature
	Debugf("Handshake: reading client version and signature\n")
	if _, err := io.ReadFull(conn.org, buf1); err != nil {
		return fmt.Errorf("read client version and signature: %w", err)
	}

	if buf1[0] != allowedProtoVer {
		return errUnsupportedProtoVer
	}

	conn.epoch = binary.BigEndian.Uint32(buf1[1:])

	// Send server version and signature
	buf2[0] = allowedProtoVer
	epoch := uint32(0)
	binary.BigEndian.PutUint32(buf2[1:], epoch)

	Debugf("Handshake: generating server random data\n")
	if _, err := rand.Read(buf2[9:]); err != nil {
		return fmt.Errorf("generate server random data: %w", err)
	}

	Debugf("Handshake: sending server version and signature\n")
	if _, err := conn.org.Write(buf2); err != nil {
		return fmt.Errorf("send server version and signature: %w", err)
	}

	// Echo client signature
	Debugf("Handshake: echoing client signature\n")
	if _, err := conn.org.Write(buf1[1:]); err != nil {
		return fmt.Errorf("echo client signature: %w", err)
	}

	srvSig := make([]byte, sigSize)

	// Read echoed server signature
	Debugf("Handshake: reading echoed server signature\n")
	if _, err := io.ReadFull(conn.org, srvSig); err != nil {
		return fmt.Errorf("read echoed server signature: %w", err)
	}

	if !bytes.Equal(srvSig, buf2[1:]) {
		return errSignatureMismatch
	}

	return nil
}

func handshakeClient(conn *Conn) error {
	cliData := make([]byte, sigSize+1) // +1 byte for proto version
	srvData := make([]byte, sigSize+1)

	// Send client version and signature
	cliData[0] = allowedProtoVer
	epoch := uint32(0)
	binary.BigEndian.PutUint32(cliData[1:], epoch)

	if _, err := rand.Read(cliData[9:]); err != nil {
		return fmt.Errorf("generate client random data: %w", err)
	}

	Debugf("Handshake: sending client version and signature\n")
	if _, err := conn.org.Write(cliData); err != nil {
		return fmt.Errorf("send client version and signature: %w", err)
	}

	// Read server version and signature
	Debugf("Handshake: reading server version and signature\n")
	if _, err := io.ReadFull(conn.org, srvData); err != nil {
		return fmt.Errorf("read server version and signature: %w", err)
	}

	if srvData[0] != allowedProtoVer {
		return errUnsupportedProtoVer
	}

	conn.epoch = binary.BigEndian.Uint32(srvData[1:])

	// Echo server signature
	Debugf("Handshake: echoing server signature\n")
	if _, err := conn.org.Write(srvData[1:]); err != nil {
		return fmt.Errorf("echo server signature: %w", err)
	}

	// Read echoed client signature
	Debugf("Handshake: reading echoed client signature\n")
	if _, err := io.ReadFull(conn.org, srvData[:sigSize]); err != nil {
		return fmt.Errorf("read echoed client signature: %w", err)
	}

	if !bytes.Equal(cliData[1:], srvData[:sigSize]) {
		return errSignatureMismatch
	}

	return nil
}
