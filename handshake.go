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
)

const hshakeSigSize = 1536
const hshakeRndDataSize = 1528
const allowedProtoVer uint8 = 3

func handshakeServer(conn *Conn) error {
	cliData := make([]byte, hshakeSigSize+1)
	srvData := make([]byte, hshakeSigSize+1)
	cliRndData := make([]byte, hshakeRndDataSize)

	// Read client version and signature
	if _, err := io.ReadFull(conn.org, cliData); err != nil {
		return fmt.Errorf("read client version and signature: %w", err)
	}

	if cliData[0] != allowedProtoVer {
		return errUnsupportedProtoVer
	}

	conn.epoch = binary.BigEndian.Uint32(cliData[1:])
	copy(cliRndData, cliData[9:])

	// Send server version and signature
	srvData[0] = allowedProtoVer
	epoch := uint32(0)
	binary.BigEndian.PutUint32(srvData[1:], epoch)

	if _, err := rand.Read(srvData[9:]); err != nil {
		return fmt.Errorf("generate server random data: %w", err)
	}

	if _, err := conn.org.Write(srvData); err != nil {
		return fmt.Errorf("send server version and signature: %w", err)
	}

	// Read echoed server signature
	if _, err := io.ReadFull(conn.org, cliData[:hshakeSigSize]); err != nil {
		return fmt.Errorf("read echoed server signature: %w", err)
	}

	if !bytes.Equal(cliData[:hshakeSigSize], srvData[1:]) {
		return fmt.Errorf("signature mismatch")
	}

	// Echo client signature
	binary.BigEndian.PutUint32(srvData, conn.epoch)
	binary.BigEndian.PutUint32(srvData[4:], 0)
	copy(srvData[8:], cliRndData)

	if _, err := conn.org.Write(srvData[:hshakeSigSize]); err != nil {
		return fmt.Errorf("echo client signature: %w", err)
	}

	return nil
}

func handshakeClient(conn *Conn) error {
	cliData := make([]byte, hshakeSigSize+1) // +1 byte for proto version
	srvData := make([]byte, hshakeSigSize+1)

	// Send client version and signature
	cliData[0] = allowedProtoVer
	epoch := uint32(0)
	binary.BigEndian.PutUint32(cliData[1:], epoch)

	if _, err := rand.Read(cliData[9:]); err != nil {
		return fmt.Errorf("generate client random data: %w", err)
	}

	if _, err := conn.org.Write(cliData); err != nil {
		return fmt.Errorf("send client version and signature: %w", err)
	}

	// Read server version and signature
	if _, err := io.ReadFull(conn.org, srvData); err != nil {
		return fmt.Errorf("read server version and signature: %w", err)
	}

	if srvData[0] != allowedProtoVer {
		return errUnsupportedProtoVer
	}

	conn.epoch = binary.BigEndian.Uint32(srvData[1:])

	// Echo server signature
	if _, err := conn.org.Write(srvData[1:]); err != nil {
		return fmt.Errorf("echo server signature: %w", err)
	}

	// Read echoed client signature
	if _, err := io.ReadFull(conn.org, srvData[:hshakeSigSize]); err != nil {
		return fmt.Errorf("read echoed client signature: %w", err)
	}

	if !bytes.Equal(cliData[1:], srvData[:hshakeSigSize]) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
