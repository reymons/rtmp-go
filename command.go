package rtmp

import (
	"errors"
	"fmt"

	"rtmp/amf"
)

var (
	ErrInvalidCommandLabel = errors.New("invalid commang label")
)

const (
	cmdInitial uint8 = iota

	CmdResult
	CmdError
	CmdInform

	CmdConnect
	CmdCreateStream
	CmdDeleteStream
	CmdPublishStream

	CmdUnsupported
)

var cmdToName = map[uint8]string{
	CmdResult: "_result",
	CmdError:  "_error",
	CmdInform: "onStatus",

	CmdConnect:       "connect",
	CmdCreateStream:  "createStream",
	CmdDeleteStream:  "deleteStream",
	CmdPublishStream: "publish",
}

var nameToCmd = reverseMap(cmdToName)

type CommandMessage interface {
	Message

	Encode(enc *amf.Encoder) error

	Decode(dec *amf.Decoder) error

	Header() *CommandHeader

	setTrx(trx uint32)
	setLabel(label uint8)
}

type CommandHeader struct {
	label uint8
	trx   uint32
}

func (h *CommandHeader) Label() uint8 {
	return h.label
}

func (h *CommandHeader) Trx() uint32 {
	return h.trx
}

func (h *CommandHeader) Encode(enc *amf.Encoder) error {
	label, ok := cmdToName[h.label]
	if !ok {
		return ErrInvalidCommandLabel
	}
	enc.PutString(label)
	enc.PutUint32(h.trx)
	return nil
}

func (h *CommandHeader) Decode(dec *amf.Decoder) error {
	if h.label != cmdInitial {
		return nil
	}
	labelStr, err := dec.GetString()
	if err != nil {
		return fmt.Errorf("get command label: %w", err)
	}
	label, ok := nameToCmd[labelStr]
	if !ok {
		// Due to legacy commands and yet-not-implemented ones, mark label as unsupported
		label = CmdUnsupported
	}
	trx, err := dec.GetUint32()
	if err != nil {
		return fmt.Errorf("get command transaction ID: %w", err)
	}
	h.label = label
	h.trx = trx
	return nil
}

type cmdmesg struct {
	hdr CommandHeader
}

func (m *cmdmesg) Header() *CommandHeader {
	return &m.hdr
}

func (m *cmdmesg) setLabel(label uint8) {
	m.hdr.label = label
}

func (m *cmdmesg) setTrx(trx uint32) {
	m.hdr.trx = trx
}

type ConnectMessage struct {
	cmdmesg
	AppName string
}

func (m *ConnectMessage) Encode(enc *amf.Encoder) error {
	m.cmdmesg.setLabel(CmdConnect)
	if err := m.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	props := amf.NewObject()
	props.AddProp("app", m.AppName)
	enc.PutObject(&props)
	enc.PutNull()
	return nil
}

func (m *ConnectMessage) Decode(dec *amf.Decoder) error {
	if err := m.cmdmesg.Header().Decode(dec); err != nil {
		return err
	}
	props, err := dec.GetObject()
	if err != nil {
		return fmt.Errorf("read props: %w", err)
	}
	m.AppName = props.Prop("app").Value().String()
	return nil
}

type CreateStreamMessage struct {
	cmdmesg
}

func (m *CreateStreamMessage) Encode(enc *amf.Encoder) error {
	m.cmdmesg.setLabel(CmdCreateStream)
	if err := m.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	enc.PutNull()
	return nil
}

func (m *CreateStreamMessage) Decode(dec *amf.Decoder) error {
	if err := m.cmdmesg.Header().Decode(dec); err != nil {
		return err
	}
	return nil
}

type PublishStreamMessage struct {
	cmdmesg
	PublishingName string
	PublishingType string
}

func (m *PublishStreamMessage) Encode(enc *amf.Encoder) error {
	m.cmdmesg.setLabel(CmdPublishStream)
	if err := m.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	enc.PutNull()
	enc.PutString(m.PublishingName)
	enc.PutString(m.PublishingType)
	return nil
}

func (m *PublishStreamMessage) Decode(dec *amf.Decoder) error {
	if err := m.cmdmesg.Header().Decode(dec); err != nil {
		return err
	}
	dec.SkipNull()
	name, err := dec.GetString()
	if err != nil {
		return fmt.Errorf("get publishing name: %w", err)
	}
	typ, err := dec.GetString()
	if err != nil {
		return fmt.Errorf("get publishing type: %w", err)
	}
	m.PublishingName = name
	m.PublishingType = typ
	return nil
}

type CloseStreamMessage struct {
	cmdmesg
	Stream uint32
}

func (m *CloseStreamMessage) Encode(enc *amf.Encoder) error {
	m.cmdmesg.setLabel(CmdDeleteStream)
	if err := m.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	enc.PutNull()
	enc.PutUint32(m.Stream)
	return nil
}

func (m *CloseStreamMessage) Decode(dec *amf.Decoder) error {
	if err := m.cmdmesg.Header().Decode(dec); err != nil {
		return err
	}
	dec.SkipNull()
	stream, err := dec.GetUint32()
	if err != nil {
		return fmt.Errorf("get stream ID: %w", err)
	}
	m.Stream = stream
	return nil
}
