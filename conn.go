package rtmp

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"time"
	"unsafe"

	"github.com/reymons/amf-go"
)

var (
	ErrUnsupportedMessage     = errors.New("unsupported message")
	ErrInvalidStream          = errors.New("invalid stream")
	ErrMaxChannels            = errors.New("maximum number of active channels has been reached")
	ErrInvalidChunkSize       = errors.New("invalid chunk size")
	ErrInvalidBandwidthType   = errors.New("invalid bandwidth type")
	ErrConnClosed             = errors.New("connection closed")
	ErrNonMediaPacketTooLarge = errors.New("non-media packet is too large")
)

func getMaxMesgSize(messages ...Message) uintptr {
	var size uintptr
	for _, mesg := range messages {
		mesgSize := reflect.TypeOf(mesg).Size()
		if mesgSize > size {
			size = mesgSize
		}
	}
	return size
}

var mesgBufSize = getMaxMesgSize(
	// Basic messages
	VideoMessage{},
	AudioMessage{},
	// Command messages
	ConnectMessage{},
	CreateStreamMessage{},
	PublishStreamMessage{},
	CloseStreamMessage{},
	// Data messages
	MetaDataMessage{},
)

const defaultChunkSize uint32 = 128
const defaultWinSize = 25000000
const maxChannels = 10
const readDeadline = time.Second * 15
const maxChunkSize = 128 * 1024
const maxNonMediaPackLen = 4096
const ctrlChannel = 2

const ControlStream = 0

type Conn struct {
	org            net.Conn
	reader         *connReader
	epoch          uint32
	recvChunkSize  uint32
	sendChunkSize  uint32
	channels       map[uint32]*chunkChannel
	sendChannels   map[uint32]*sendChunkChannel
	sendBuf        []byte
	mesgBuf        []uintptr // use uintptr for alignment
	sendWinSize    uint32
	recvWinSize    uint32
	totalRecvBytes uint32
	lastAckedBytes uint32
	nextStream     uint32
	dec            *amf.Decoder
	enc            *amf.Encoder
}

type connReader struct {
	conn   *Conn
	reader *bufio.Reader
}

func (r *connReader) Read(data []byte) (int, error) {
	if err := r.conn.org.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		return 0, err
	}

	n, err := r.reader.Read(data)
	r.conn.totalRecvBytes += uint32(n)

	unackedBytes := r.conn.totalRecvBytes - r.conn.lastAckedBytes
	if unackedBytes >= r.conn.recvWinSize {
		if err := r.conn.sendAck(); err != nil {
			return 0, fmt.Errorf("send ack: %w", err)
		}
		r.conn.lastAckedBytes = r.conn.totalRecvBytes
	}

	return n, err
}

func newConn(org net.Conn) *Conn {
	conn := &Conn{
		org:           org,
		recvChunkSize: defaultChunkSize,
		channels:      make(map[uint32]*chunkChannel),
		sendChannels:  make(map[uint32]*sendChunkChannel),
		mesgBuf:       make([]uintptr, mesgBufSize),
		sendWinSize:   defaultWinSize,
		recvWinSize:   defaultWinSize,
		nextStream:    1,
		enc:           amf.NewEncoder(),
		dec:           amf.NewDecoder(),
	}

	conn.reader = &connReader{
		conn:   conn,
		reader: bufio.NewReaderSize(org, maxChunkHdrSize),
	}
	conn.setSendChunkSize(defaultChunkSize)

	return conn
}

func (conn *Conn) LocalAddr() net.Addr {
	return conn.org.LocalAddr()
}

func (conn *Conn) RemoteAddr() net.Addr {
	return conn.org.RemoteAddr()
}

func (conn *Conn) Close() error {
	return conn.org.Close()
}

func (conn *Conn) sendAck() error {
	Debugf("Sending ack: %d\n", conn.totalRecvBytes)
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, conn.totalRecvBytes)
	err := conn.sendControlPack(PackAck, data)
	if err != nil {
		Debugf("Sending ack failed: %v\n", err)
	}
	return err
}

func (conn *Conn) sendControlPack(typ uint8, data []byte) error {
	return conn.SendPacket(&Packet{
		Type:    typ,
		Channel: ctrlChannel,
		Stream:  ControlStream,
		DataRaw: data,
	})
}

func (conn *Conn) sendWinAckSize(size uint32) error {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, size)
	return conn.sendControlPack(PackWinAckSize, data)
}

func (conn *Conn) setSendChunkSize(size uint32) {
	conn.sendChunkSize = size
	conn.sendBuf = make([]byte, size*maxChunkHdrSize)
}

func (conn *Conn) SetChunkSize(size uint32) error {
	if size < 1 || size > maxChunkSize {
		return ErrInvalidChunkSize
	}
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, size)
	if err := conn.sendControlPack(PackSetChunkSize, data); err != nil {
		return err
	}
	conn.setSendChunkSize(size)
	return nil
}

func (conn *Conn) onSetChunkSize(pack *Packet) error {
	data := make([]byte, 4)
	if _, err := io.ReadFull(pack.Data, data); err != nil {
		return err
	}

	size := binary.BigEndian.Uint32(data)
	if size < 1 || size > maxChunkSize {
		return ErrInvalidChunkSize
	}
	Debugf("New receive chunk size: %d\n", size)
	conn.recvChunkSize = size

	return nil
}

func (conn *Conn) AbortMessage(csid uint32) error {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, csid)
	return conn.sendControlPack(PackAbort, data)
}

func (conn *Conn) onAbort(pack *Packet) error {
	data := make([]byte, 4)
	if _, err := io.ReadFull(pack.Data, data); err != nil {
		return err
	}

	id := binary.BigEndian.Uint32(data)
	if channel := conn.channels[id]; channel != nil {
		// TODO: restore channel's state as it was before accepting the first chunk of the current message
	}

	return nil
}

func (conn *Conn) SetWinAckSize(size uint32) error {
	err := conn.sendWinAckSize(size)
	if err == nil {
		Debugf("New send win ack size: %d\n", size)
		conn.sendWinSize = size
	}
	return err
}

func (conn *Conn) onWinAckSize(pack *Packet) error {
	data := make([]byte, 4)
	if _, err := io.ReadFull(pack.Data, data); err != nil {
		return err
	}
	conn.recvWinSize = binary.BigEndian.Uint32(pack.DataRaw)
	Debugf("New receive win ack size: %d\n", conn.recvWinSize)
	return nil
}

const (
	BWHard = iota
	BWSoft
	BWDynamic
	bwTotal
)

func (conn *Conn) SetBandwidth(size uint32, typ uint8) error {
	data := make([]byte, 5)
	binary.BigEndian.PutUint32(data, size)
	data[4] = typ
	if err := conn.sendControlPack(PackSetBandwidth, data); err != nil {
		return err
	}
	Debugf("New receive win size: %d\n", size)
	conn.recvWinSize = size
	return nil
}

func (conn *Conn) onSetBandwidth(pack *Packet) error {
	data := make([]byte, 5)
	if _, err := io.ReadFull(pack.Data, data); err != nil {
		return err
	}

	typ := data[4]
	if typ >= bwTotal {
		return ErrInvalidBandwidthType
	}

	size := binary.BigEndian.Uint32(data)
	newSize := size

	if typ == BWSoft && size > conn.sendWinSize {
		newSize = conn.sendWinSize
	}

	if newSize != conn.sendWinSize {
		err := conn.sendWinAckSize(newSize)
		if err == nil {
			Debugf("New send win size: %d\n", size)
			conn.sendWinSize = newSize
		}
		return err
	}

	return nil
}

func (conn *Conn) onAck(pack *Packet) error {
	data := make([]byte, 4)
	if _, err := io.ReadFull(pack.Data, data); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(data)
	Debugf("Received ack: %d\n", size)
	return nil
}

func (conn *Conn) onControlPacket(pack *Packet) (bool, error) {
	var err error
	var handled = true

	switch pack.Type {
	case PackSetChunkSize:
		err = conn.onSetChunkSize(pack)
	case PackAbort:
		err = conn.onAbort(pack)
	case PackWinAckSize:
		err = conn.onWinAckSize(pack)
	case PackSetBandwidth:
		err = conn.onSetBandwidth(pack)
	case PackAck:
		err = conn.onAck(pack)
	default:
		handled = false
	}

	return handled, err
}

func (conn *Conn) getOrCreateChannel(id uint32) (*chunkChannel, error) {
	channel := conn.channels[id]
	if channel == nil {
		if len(conn.channels) >= maxChannels {
			return nil, ErrMaxChannels
		}
		channel = &chunkChannel{id: id, conn: conn}
		conn.channels[id] = channel
	}
	return channel, nil
}

// Returns a channel that handled the read chunk
func (conn *Conn) readChunk() (*chunkChannel, error) {
	var chunk rtmpChunk
	if err := chunk.decode(conn.reader); err != nil {
		return nil, fmt.Errorf("decode chunk: %w", err)
	}
	Debugf("NEW CHUNK: %+v", chunk)
	channel, err := conn.getOrCreateChannel(chunk.channel)
	if err != nil {
		return nil, fmt.Errorf("get or create channel: %w", err)
	}
	channel.HandleChunk(&chunk)
	return channel, nil
}

func (conn *Conn) ReadPacket(pack *Packet) error {
	for {
		channel, err := conn.readChunk()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ErrConnClosed
			}
			return fmt.Errorf("read chunk: %w", err)
		}

		pack.Channel = channel.id
		pack.Stream = channel.packStream
		pack.Type = channel.packType
		pack.Timestamp = channel.timestamp
		pack.Length = channel.packLength
		pack.Data = channel

		Debugf("NEW PACKET: %+v", pack)

		if pack.Stream == ControlStream {
			handled, err := conn.onControlPacket(pack)
			if err != nil {
				return fmt.Errorf("handle control message: %w", err)
			}
			if handled {
				continue
			}
		}

		return nil
	}
}

func (conn *Conn) packetToDataMesg(pack *Packet) (DataMessage, error) {
	data, err := io.ReadAll(pack.Data)
	if err != nil {
		return nil, fmt.Errorf("read packet data: %w", err)
	}

	conn.dec.SetData(data)
	cmd, err := conn.dec.GetString()
	if err != nil {
		return nil, fmt.Errorf("decode data message command: %w", err)
	}

	buf := unsafe.Pointer(&conn.mesgBuf[0])
	var mesg DataMessage

	Debugf("NEW DATA MESSAGE: %s", cmd)

	switch cmd {
	case DataMesgSetDataFrame:
		event, err := conn.dec.GetString()
		if err != nil {
			return nil, fmt.Errorf("decode data message event: %w", err)
		}

		Debugf("NEW DATA MESSAGE EVENT: %s", event)

		switch event {
		case DataMesgEventMetaData:
			mesg = (*MetaDataMessage)(buf)
		default:
			return nil, ErrUnsupportedMessage
		}

	default:
		return nil, ErrUnsupportedMessage
	}

	if err := mesg.Decode(conn.dec); err != nil {
		return nil, fmt.Errorf("decode data message: %w", err)
	}
	return mesg, nil
}

func (conn *Conn) packetToCmdMesg(pack *Packet) (CommandMessage, error) {
	data, err := io.ReadAll(pack.Data)
	if err != nil {
		return nil, fmt.Errorf("read packet data: %w", err)
	}
	conn.dec.SetData(data)

	hdr := CommandHeader{}
	if err := hdr.Decode(conn.dec); err != nil {
		return nil, fmt.Errorf("decode command header: %w", err)
	}

	buf := unsafe.Pointer(&conn.mesgBuf[0])

	var mesg CommandMessage

	Debugf("NEW CMD MESSAGE: %s", hdr.LabelString())

	switch hdr.Label() {
	case CmdConnect:
		mesg = (*ConnectMessage)(buf)
	case CmdCreateStream:
		mesg = (*CreateStreamMessage)(buf)
	case CmdPublishStream:
		mesg = (*PublishStreamMessage)(buf)
	case CmdDeleteStream:
		mesg = (*CloseStreamMessage)(buf)
	default:
		return nil, ErrUnsupportedMessage
	}

	mesg.setLabel(hdr.Label())
	mesg.setTrx(hdr.Trx())

	if err := mesg.Decode(conn.dec); err != nil {
		return nil, fmt.Errorf("decode command message: %w", err)
	}
	return mesg, nil
}

func (conn *Conn) packetToBasicMesg(pack *Packet) (BasicMessage, error) {
	buf := unsafe.Pointer(&conn.mesgBuf[0])

	var mesg BasicMessage

	switch pack.Type {
	case PackVideo:
		mesg = (*VideoMessage)(buf)
	case PackAudio:
		mesg = (*AudioMessage)(buf)
	default:
		return nil, ErrUnsupportedMessage
	}

	mesg.FromPacket(pack)
	return mesg, nil
}

func (conn *Conn) ReadStreamMessage(stream uint32) (Message, error) {
	mesg, mesgStream, err := conn.ReadMessage()
	if err == nil && mesgStream != stream && mesgStream != ControlStream {
		return nil, ErrInvalidStream
	}
	return mesg, err
}

func (conn *Conn) ReadMessage() (Message, uint32, error) {
	var pack Packet
	if err := conn.ReadPacket(&pack); err != nil {
		if err == ErrConnClosed {
			return nil, 0, err
		}
		return nil, 0, fmt.Errorf("read packet: %w", err)
	}

	if pack.Type != PackVideo && pack.Type != PackAudio && pack.Length >= maxNonMediaPackLen {
		return nil, 0, ErrNonMediaPacketTooLarge
	}

	var mesg Message
	var err error

	if pack.Type == PackCmdAMF0 {
		mesg, err = conn.packetToCmdMesg(&pack)
	} else if pack.Type == PackDataAMF0 {
		mesg, err = conn.packetToDataMesg(&pack)
	} else {
		mesg, err = conn.packetToBasicMesg(&pack)
	}

	if err == ErrUnsupportedMessage {
		if err := pack.discard(); err != nil {
			return nil, 0, fmt.Errorf("discard packet data: %w", err)
		}
	}

	return mesg, pack.Stream, err
}

func (conn *Conn) SendPacket(pack *Packet) error {
	chunkType := chunkLargest
	packLength := uint32(len(pack.DataRaw))
	timestamp := pack.Timestamp

	{
		channel := conn.sendChannels[pack.Channel]
		newChannel := channel == nil

		if newChannel {
			if len(conn.sendChannels) >= maxChannels {
				return ErrMaxChannels
			}
			channel = &sendChunkChannel{id: pack.Channel}
			conn.sendChannels[pack.Channel] = channel
		}

		if newChannel || pack.Stream != channel.packStream || channel.timestamp > timestamp {
			chunkType = chunkLargest
			channel.packStream = pack.Stream
			channel.packType = pack.Type
			channel.packLength = packLength
			channel.timestamp = timestamp
		} else {
			chunkType = chunkSmall
			if pack.Type != channel.packType || packLength != channel.packLength {
				chunkType = chunkLarge
				channel.packType = pack.Type
				channel.packLength = packLength
			}
			timestamp -= channel.timestamp
			channel.timestamp = pack.Timestamp
		}
	}

	var sent uint32

	for sent < packLength {
		chunk := rtmpChunk{
			chunkType: chunkType,
			channel:   pack.Channel,
		}

		switch chunk.chunkType {
		case chunkLargest:
			chunk.timestamp = timestamp
			chunk.packStream = pack.Stream
			chunk.packType = pack.Type
			chunk.packLength = packLength
		case chunkLarge:
			chunk.timestamp = timestamp
			chunk.packType = pack.Type
			chunk.packLength = packLength
		case chunkSmall:
			chunk.timestamp = timestamp
		case chunkSmallest:
			// do nothing
		}
		chunkType = chunkSmallest

		n, err := chunk.encode(conn.sendBuf)
		if err != nil {
			return fmt.Errorf("encode chunk: %w", err)
		}

		// TODO: buffer small writes, for now just do two syscalls
		hdr := conn.sendBuf[:n]
		if _, err := conn.org.Write(hdr); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				err = ErrConnClosed
			} else {
				err = fmt.Errorf("write chunk header: %w", err)
			}
			return err
		}

		chunkSize := packLength - sent
		if chunkSize > conn.sendChunkSize {
			chunkSize = conn.sendChunkSize
		}

		data := pack.DataRaw[sent : sent+chunkSize]
		if _, err := conn.org.Write(data); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				err = ErrConnClosed
			} else {
				err = fmt.Errorf("write chunk data: %w", err)
			}
			return err
		}
		sent += chunkSize
	}

	return nil
}

func (conn *Conn) SendCommandPacket(stream, channel uint32, data []byte) error {
	pack := Packet{
		Stream:  stream,
		Channel: channel,
		Type:    PackCmdAMF0,
		DataRaw: data,
	}
	return conn.SendPacket(&pack)
}

func (conn *Conn) SendCommandMessage(mesg CommandMessage, trx, stream, channel uint32) error {
	mesg.setTrx(trx)
	conn.enc.Reset()
	if err := mesg.Encode(conn.enc); err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	if err := conn.SendCommandPacket(stream, channel, conn.enc.Data()); err != nil {
		return fmt.Errorf("send packet: %w", err)
	}
	return nil
}

func (conn *Conn) SendCommandMessageReply(reply CommandMessageReply, label uint8, trx, stream, channel uint32) error {
	reply.setLabel(label)
	return conn.SendCommandMessage(reply, trx, stream, channel)
}

func (conn *Conn) ReadCommandMessageReply(reply CommandMessageReply, trx uint32) error {
	for {
		var pack Packet
		if err := conn.ReadPacket(&pack); err != nil {
			return fmt.Errorf("read packet: %w", err)
		}

		if pack.Type != PackCmdAMF0 {
			if err := pack.discard(); err != nil {
				return fmt.Errorf("drain packet data: %w", err)
			}
			continue
		}

		conn.dec.SetData(pack.DataRaw)
		hdr := CommandHeader{}
		if err := hdr.Decode(conn.dec); err != nil {
			return fmt.Errorf("decode command header: %w", err)
		}

		if hdr.Trx() != trx {
			// TODO: if other transaction comes in, do something
			continue
		}

		switch hdr.Label() {
		case CmdResult, CmdError, CmdInform:
			conn.dec.SetData(pack.DataRaw)
			if err := reply.Decode(conn.dec); err != nil {
				return fmt.Errorf("decode reply: %w", err)
			}
		default:
			// TODO: handle default case
		}
	}
}

func (conn *Conn) SendCommandError(trx, stream, channel uint32) error {
	hdr := CommandHeader{
		label: CmdError,
		trx:   trx,
	}
	conn.enc.Reset()
	if err := hdr.Encode(conn.enc); err != nil {
		return err
	}
	pack := Packet{
		Channel: channel,
		Stream:  stream,
		Type:    PackCmdAMF0,
		DataRaw: conn.enc.Data(),
	}
	return conn.SendPacket(&pack)
}

func (conn *Conn) ReadCommandMessage(label uint8) (CommandMessage, error) {
	for {
		var pack Packet
		if err := conn.ReadPacket(&pack); err != nil {
			return nil, err
		}

		if pack.Type != PackCmdAMF0 {
			if err := pack.discard(); err != nil {
				return nil, fmt.Errorf("drain packet data: %w", err)
			}
			continue
		}

		mesg, err := conn.packetToCmdMesg(&pack)
		if err != nil {
			if err == ErrUnsupportedMessage {
				if err := pack.discard(); err != nil {
					return nil, fmt.Errorf("drain packet data: %w", err)
				}
				continue
			}
			return nil, fmt.Errorf("packet to command message: %w", err)
		}
		if mesg.Header().Label() == label {
			return mesg, nil
		}
	}
}

type AcceptStreamOptions struct {
	UserData  any
	OnConnect func(mesg *ConnectMessage, userData any) error
	OnPublish func(mesg *PublishStreamMessage, userData any) error
}

func (conn *Conn) AcceptStream(opts *AcceptStreamOptions) (uint32, error) {
	var stream uint32

	// Connect
	{
		mesg, err := conn.ReadCommandMessage(CmdConnect)
		if err != nil {
			return 0, fmt.Errorf("read connect message: %w", err)
		}
		if err := opts.OnConnect(mesg.(*ConnectMessage), opts.UserData); err != nil {
			conn.SendCommandError(mesg.Header().Trx(), ControlStream, ctrlChannel)
			return 0, err
		}
		reply := ConnectMessageReply{
			FMSVer:       "FMS/3,5,7,7009",
			Capabilities: 31,
			Info: CommandMessageReplyInfo{
				Code:           "NetConnection.Connect.Success",
				Level:          "status",
				ObjectEncoding: 0,
			},
		}
		err = conn.SendCommandMessageReply(&reply, CmdResult, mesg.Header().Trx(), ControlStream, ctrlChannel)
		if err != nil {
			return 0, fmt.Errorf("send command packet: %w", err)
		}
	}

	// Create stream
	{
		mesg, err := conn.ReadCommandMessage(CmdCreateStream)
		if err != nil {
			return 0, fmt.Errorf("read create stream message: %w", err)
		}
		reply := CreateStreamMessageReply{Stream: conn.nextStream}
		err = conn.SendCommandMessageReply(&reply, CmdResult, mesg.Header().Trx(), ControlStream, ctrlChannel)
		if err != nil {
			return 0, fmt.Errorf("send command packet: %w", err)
		}
		stream = conn.nextStream
		conn.nextStream += 1
	}

	// Publish
	{
		mesg, err := conn.ReadCommandMessage(CmdPublishStream)
		if err != nil {
			return 0, fmt.Errorf("read create stream message: %w", err)
		}
		if err := opts.OnPublish(mesg.(*PublishStreamMessage), opts.UserData); err != nil {
			conn.SendCommandError(mesg.Header().Trx(), ControlStream, ctrlChannel)
			return 0, err
		}
		reply := PublishStreamMessageReply{
			Info: CommandMessageReplyInfo{
				Level:       "status",
				Code:        "NetStream.Play.Start",
				Description: "Start stream",
			},
		}
		err = conn.SendCommandMessageReply(&reply, CmdInform, mesg.Header().Trx(), ControlStream, ctrlChannel)
		if err != nil {
			return 0, fmt.Errorf("send command packet: %w", err)
		}
	}

	return stream, nil
}
