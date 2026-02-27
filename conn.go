package rtmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/reymons/amf-go"
)

var (
	ErrInvalidPacket        = errors.New("invalid packet")
	ErrInvalidStream        = errors.New("invalid stream")
	ErrPacketTooLarge       = errors.New("packet is too large")
	ErrChunkDataTooLarge    = errors.New("chunk data too large")
	ErrMaxChannels          = errors.New("maximum number of active channels has been reached")
	ErrInvalidMessageData   = errors.New("invalid message data")
	ErrInvalidChunkSize     = errors.New("invalid chunk size")
	ErrInvalidBandwidthType = errors.New("invalid bandwidth type")
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
)

type Conn struct {
	org            net.Conn
	reader         io.Reader
	epoch          uint32
	recvChunkSize  uint32
	sendChunkSize  uint32
	channels       map[uint32]*Channel
	sendChannels   map[uint32]*SendChannel
	sendBuf        []byte
	mesgBuf        []uintptr // use uintptr for alignment
	sendWinSize    uint32
	recvWinSize    uint32
	totalRecvBytes uint32
	lastAckedBytes uint32
	nextStream     uint32
	dec            *amf.Decoder
	enc            *amf.Encoder
	sendMtx        sync.Mutex

	readQueuedPackets bool
	packets           []Packet
	packetsRd         int
	packetsWr         int
}

const defaultChunkSize uint32 = 128
const defaultWinSize = 25000000
const maxChannels = 10
const readDeadline = time.Second * 15
const maxPackLength = 1024 * 1024
const maxPackLengthInQueue = 512
const packQueueSize = 10
const ctrlChannel = 2

const ControlStream = 0

type connReader struct {
	conn *Conn
}

func (r *connReader) Read(data []byte) (int, error) {
	if err := r.conn.org.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		return 0, err
	}

	n, err := r.conn.org.Read(data)
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
		org:               org,
		recvChunkSize:     defaultChunkSize,
		channels:          make(map[uint32]*Channel),
		sendChannels:      make(map[uint32]*SendChannel),
		mesgBuf:           make([]uintptr, mesgBufSize),
		sendWinSize:       defaultWinSize,
		recvWinSize:       defaultWinSize,
		nextStream:        1,
		packets:           make([]Packet, packQueueSize+1),
		readQueuedPackets: true,
		enc:               amf.NewEncoder(),
		dec:               amf.NewDecoder(),
	}

	conn.reader = &connReader{conn}
	conn.setSendChunkSize(defaultChunkSize)

	return conn
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

func (conn *Conn) readQueuedPacket(pack *Packet) bool {
	if conn.packetsRd == conn.packetsWr {
		return false
	}
	*pack = conn.packets[conn.packetsRd]
	conn.packetsRd = (conn.packetsRd + 1) % len(conn.packets)
	return true
}

func (conn *Conn) queuePacket(pack *Packet) bool {
	if len(pack.Data) > maxPackLengthInQueue {
		return false
	}
	if conn.packetsRd == (conn.packetsWr+1)%len(conn.packets) {
		return false
	}
	if pack.Type == PackUserCtrl {
		// I'm not even sure if I need to buffer anything expect for user control messages
		//
		// The whole packet queueing thingy should rarely come into play,
		// so it's okay to allocate a buffer like that for now
		//
		// TODO: use buffer pool?
		data := make([]byte, len(pack.Data))
		copy(data, pack.Data)
		conn.packets[conn.packetsWr] = *pack
		conn.packets[conn.packetsWr].Data = data
		conn.packetsWr = (conn.packetsWr + 1) % len(conn.packets)
	}
	return true
}

// If a message's been fully read, returns a non-nil channel that handled it
func (conn *Conn) readChunk() (*Channel, error) {
	var chunk Chunk
	if err := chunk.Decode(conn.reader); err != nil {
		return nil, fmt.Errorf("decode chunk: %w", err)
	}

	if (chunk.Type == chunkLargest || chunk.Type == chunkLarge) && chunk.PackLength > maxPackLength {
		// TODO: maybe skip the message entirely by reading out the remaining chunks?
		return nil, ErrPacketTooLarge
	}

	// Get a channel, otherwise create it
	channel := conn.channels[chunk.Channel]
	if channel == nil {
		if len(conn.channels) >= maxChannels {
			return nil, ErrMaxChannels
		}
		channel = &Channel{ID: chunk.Channel}
		conn.channels[chunk.Channel] = channel
	}

	switch chunk.Type {
	case chunkLargest:
		channel.Timestamp = chunk.Timestamp
		channel.TimestampDelta = 0
		channel.PackStream = chunk.PackStream
		channel.PackLength = chunk.PackLength
		channel.PackType = chunk.PackType
		channel.RecvBytes = 0
	case chunkLarge:
		channel.TimestampDelta = chunk.Timestamp
		channel.PackLength = chunk.PackLength
		channel.PackType = chunk.PackType
		channel.RecvBytes = 0
	case chunkSmall:
		channel.TimestampDelta = chunk.Timestamp
	case chunkSmallest:
		// do nothing
	}

	if channel.PackLength > uint32(len(channel.RecvBuf)) {
		size := uint32(len(channel.RecvBuf)) + 128
		for size < channel.PackLength {
			size *= 2
		}
		channel.RecvBuf = make([]byte, size)
	}

	chunkSize := channel.PackLength - channel.RecvBytes
	if chunkSize > conn.recvChunkSize {
		chunkSize = conn.recvChunkSize
	}

	rb := channel.RecvBytes
	data := channel.RecvBuf[rb : rb+chunkSize]
	if _, err := io.ReadFull(conn.reader, data); err != nil {
		return nil, fmt.Errorf("read chunk payload: %w", err)
	}
	channel.RecvBytes += uint32(len(data))

	if channel.RecvBytes >= channel.PackLength {
		channel.Timestamp += channel.TimestampDelta
		channel.RecvBytes = 0
		return channel, nil
	}

	return nil, nil
}

func (conn *Conn) sendControlPack(typ uint8, data []byte) error {
	return conn.SendPacket(&Packet{
		Type:    typ,
		Channel: ctrlChannel,
		Stream:  ControlStream,
		Data:    data,
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
	if size < 1 || size > maxPackLength {
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
	if len(pack.Data) != 4 {
		return ErrInvalidMessageData
	}

	size := binary.BigEndian.Uint32(pack.Data)
	if size < 1 || size > maxPackLength {
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
	if len(pack.Data) != 4 {
		return ErrInvalidMessageData
	}

	id := binary.BigEndian.Uint32(pack.Data)
	if channel := conn.channels[id]; channel != nil {
		channel.RecvBytes = 0
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
	if len(pack.Data) != 4 {
		return ErrInvalidMessageData
	}
	conn.recvWinSize = binary.BigEndian.Uint32(pack.Data)
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
	if len(pack.Data) != 5 {
		return ErrInvalidMessageData
	}

	typ := pack.Data[4]
	if typ >= bwTotal {
		return ErrInvalidBandwidthType
	}

	size := binary.BigEndian.Uint32(pack.Data)
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
	if len(pack.Data) != 4 {
		return ErrInvalidMessageData
	}
	size := binary.BigEndian.Uint32(pack.Data)
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

func (conn *Conn) ReadPacket(pack *Packet) error {
	for {
		if conn.readQueuedPackets && conn.readQueuedPacket(pack) {
			return nil
		}

		channel, err := conn.readChunk()
		if err != nil {
			return fmt.Errorf("read chunk: %w", err)
		}

		if channel == nil {
			continue
		}

		pack.Channel = channel.ID
		pack.Stream = channel.PackStream
		pack.Type = channel.PackType
		pack.Timestamp = channel.Timestamp
		pack.Data = channel.RecvBuf[:channel.PackLength]

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

func (conn *Conn) packetToCmdMesg(pack *Packet) (CommandMessage, error) {
	if pack.Type != PackCmdAMF0 {
		return nil, ErrInvalidPacket
	}

	conn.dec.SetData(pack.Data)

	hdr := CommandHeader{}
	if err := hdr.Decode(conn.dec); err != nil {
		return nil, fmt.Errorf("decode command header: %w", err)
	}

	buf := unsafe.Pointer(&conn.mesgBuf[0])

	var mesg CommandMessage

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
		return nil, ErrInvalidPacket
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
		return nil, ErrInvalidPacket
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
		return nil, 0, err
	}

	var mesg Message
	var err error

	if pack.Type == PackCmdAMF0 {
		mesg, err = conn.packetToCmdMesg(&pack)
	} else {
		mesg, err = conn.packetToBasicMesg(&pack)
	}

	return mesg, pack.Stream, err
}

func (conn *Conn) SendPacket(pack *Packet) error {
	chunkType := chunkLargest
	packLength := uint32(len(pack.Data))
	timestamp := pack.Timestamp

	conn.sendMtx.Lock()
	{
		channel := conn.sendChannels[pack.Channel]
		newChannel := channel == nil

		if newChannel {
			if len(conn.sendChannels) >= maxChannels {
				conn.sendMtx.Unlock()
				return ErrMaxChannels
			}
			channel = &SendChannel{ID: pack.Channel}
			conn.sendChannels[pack.Channel] = channel
		}

		if newChannel || pack.Stream != channel.PackStream || channel.Timestamp > timestamp {
			chunkType = chunkLargest
			channel.PackStream = pack.Stream
			channel.PackType = pack.Type
			channel.PackLength = packLength
			channel.Timestamp = timestamp
		} else {
			chunkType = chunkSmall
			if pack.Type != channel.PackType || packLength != channel.PackLength {
				chunkType = chunkLarge
				channel.PackType = pack.Type
				channel.PackLength = packLength
			}
			timestamp -= channel.Timestamp
			channel.Timestamp = pack.Timestamp
		}
	}
	conn.sendMtx.Unlock()

	var sent uint32

	for sent < packLength {
		conn.sendMtx.Lock()
		chunk := Chunk{Type: chunkType, Channel: pack.Channel}

		switch chunk.Type {
		case chunkLargest:
			chunk.Timestamp = timestamp
			chunk.PackStream = pack.Stream
			chunk.PackType = pack.Type
			chunk.PackLength = packLength
		case chunkLarge:
			chunk.Timestamp = timestamp
			chunk.PackType = pack.Type
			chunk.PackLength = packLength
		case chunkSmall:
			chunk.Timestamp = timestamp
		case chunkSmallest:
			// do nothing
		}
		chunkType = chunkSmallest

		n, err := chunk.Encode(conn.sendBuf)
		if err != nil {
			conn.sendMtx.Unlock()
			return fmt.Errorf("encode chunk: %w", err)
		}

		// TODO: buffer small writes, for now just do two syscalls
		hdr := conn.sendBuf[:n]
		if _, err := conn.org.Write(hdr); err != nil {
			conn.sendMtx.Unlock()
			return fmt.Errorf("write chunk header: %w", err)
		}

		chunkSize := packLength - sent
		if chunkSize > conn.sendChunkSize {
			chunkSize = conn.sendChunkSize
		}

		data := pack.Data[sent : sent+chunkSize]
		if _, err := conn.org.Write(data); err != nil {
			conn.sendMtx.Unlock()
			return fmt.Errorf("write chunk data: %w", err)
		}
		sent += chunkSize
		conn.sendMtx.Unlock()
	}

	return nil
}

func (conn *Conn) SendCommandPacket(stream, channel uint32, data []byte) error {
	pack := Packet{
		Stream:  stream,
		Channel: channel,
		Type:    PackCmdAMF0,
		Data:    data,
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

func (conn *Conn) SendCommandReply(reply CommandReply, label uint8, trx, stream, channel uint32) error {
	reply.setLabel(label)
	return conn.SendCommandMessage(reply, trx, stream, channel)
}

func (conn *Conn) ReadCommandReply(reply CommandReply, trx uint32) error {
	defer func() {
		conn.readQueuedPackets = true
	}()
	conn.readQueuedPackets = false

	for {
		var pack Packet
		if err := conn.ReadPacket(&pack); err != nil {
			return fmt.Errorf("read packet: %w", err)
		}

		if pack.Type != PackCmdAMF0 {
			if !conn.queuePacket(&pack) {
				fmt.Printf("failed to queue a packet, skipping...\n")
			}
			continue
		}

		conn.dec.SetData(pack.Data)
		hdr := CommandHeader{}
		if err := hdr.Decode(conn.dec); err != nil {
			return fmt.Errorf("decode command header: %w", err)
		}

		if hdr.Trx() != trx {
			if !conn.queuePacket(&pack) {
				fmt.Printf("failed to queue a packet, skipping...\n")
			}
			continue
		}

		switch hdr.Label() {
		case CmdResult, CmdError, CmdInform:
			conn.dec.SetData(pack.Data)
			if err := reply.Decode(conn.dec); err != nil {
				return fmt.Errorf("decode reply: %w", err)
			}
		default:
			if !conn.queuePacket(&pack) {
				fmt.Printf("failed to queue a packet, skipping...\n")
			}
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
		Data:    conn.enc.Data(),
	}
	return conn.SendPacket(&pack)
}

func (conn *Conn) ReadCommandMessage(label uint8) (CommandMessage, error) {
	conn.readQueuedPackets = false

	defer func(conn *Conn) {
		conn.readQueuedPackets = true
	}(conn)

	for {
		var pack Packet
		if err := conn.ReadPacket(&pack); err != nil {
			return nil, err
		}

		if pack.Type != PackCmdAMF0 {
			if !conn.queuePacket(&pack) {
				fmt.Printf("failed to enqueue a packet, skipping: %+v\n", pack)
			}
			continue
		}

		mesg, err := conn.packetToCmdMesg(&pack)
		if err != nil {
			if errors.Is(err, ErrInvalidPacket) {
				if !conn.queuePacket(&pack) {
					fmt.Printf("failed to enqueue a packet, skipping: %+v\n", pack)
				}
				continue
			}
			return nil, fmt.Errorf("packet to command message: %w", err)
		}
		if mesg.Header().Label() == label {
			return mesg, nil
		} else if !conn.queuePacket(&pack) {
			fmt.Printf("failed to enqueue a packet, skipping: %+v\n", pack)
		}
	}
}

type AcceptStreamOptions struct {
	OnConnect func(mesg *ConnectMessage) error
	OnPublish func(mesg *PublishStreamMessage) error
}

func (conn *Conn) AcceptStream(opts *AcceptStreamOptions) (uint32, error) {
	var stream uint32

	// Connect
	{
		mesg, err := conn.ReadCommandMessage(CmdConnect)
		if err != nil {
			return 0, fmt.Errorf("read connect message: %w", err)
		}
		if err := opts.OnConnect(mesg.(*ConnectMessage)); err != nil {
			conn.SendCommandError(mesg.Header().Trx(), ControlStream, ctrlChannel)
			return 0, err
		}
		reply := ConnectCommandReply{
			FMSVer:       "FMS/3,5,7,7009",
			Capabilities: 31,
			Info: CommandReplyInfo{
				Code:           "NetConnection.Connect.Success",
				Level:          "status",
				ObjectEncoding: 0,
			},
		}
		err = conn.SendCommandReply(&reply, CmdResult, mesg.Header().Trx(), ControlStream, ctrlChannel)
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
		reply := CreateStreamCommandReply{Stream: conn.nextStream}
		err = conn.SendCommandReply(&reply, CmdResult, mesg.Header().Trx(), ControlStream, ctrlChannel)
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
		if err := opts.OnPublish(mesg.(*PublishStreamMessage)); err != nil {
			conn.SendCommandError(mesg.Header().Trx(), ControlStream, ctrlChannel)
			return 0, err
		}
		reply := PublishStreamCommandReply{
			Info: CommandReplyInfo{
				Level:       "status",
				Code:        "NetStream.Play.Start",
				Description: "Start stream",
			},
		}
		err = conn.SendCommandReply(&reply, CmdInform, mesg.Header().Trx(), ControlStream, ctrlChannel)
		if err != nil {
			return 0, fmt.Errorf("send command packet: %w", err)
		}
	}

	return stream, nil
}
