package rtmp

import (
	"errors"
	"fmt"

	"github.com/reymons/rtmp-go/amf"
)

type CommandReplyInfo struct {
	ObjectEncoding int
	Code           string
	Level          string
	Description    string
}

var (
	ErrCmdInvalidInfoCode = errors.New("invalid code for reply info")
)

func (inf *CommandReplyInfo) Encode(enc *amf.Encoder) error {
	info := amf.NewObject()
	if inf.Level != "" {
		info.AddProp("level", inf.Level)
	}
	if inf.Description != "" {
		info.AddProp("description", inf.Description)
	}
	if inf.Code == "" {
		return ErrCmdInvalidInfoCode
	}
	info.AddProp("code", inf.Code)
	info.AddProp("objectEncoding", inf.ObjectEncoding)
	enc.PutObject(&info)
	return nil
}

func (inf *CommandReplyInfo) Decode(dec *amf.Decoder) error {
	info, err := dec.GetObject()
	if err != nil {
		return fmt.Errorf("get info object: %w", err)
	}
	inf.Code = info.Prop("code").Value().String()
	if inf.Code == "" {
		return ErrCmdInvalidInfoCode
	}
	inf.ObjectEncoding = int(info.Prop("objectEncoding").Value().Uint32())
	inf.Level = info.Prop("level").Value().String()
	inf.Description = info.Prop("description").Value().String()
	return nil
}

type CommandReply interface {
	Encode(enc *amf.Encoder) error

	Decode(dec *amf.Decoder) error

	Header() *CommandHeader

	setTrx(trx uint32)
	setLabel(label uint8)
}

type ConnectCommandReply struct {
	cmdmesg
	FMSVer       string
	Capabilities int
	Info         CommandReplyInfo
}

func (r *ConnectCommandReply) Encode(enc *amf.Encoder) error {
	if err := r.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	props := amf.NewObject()
	props.AddProp("fmsVer", r.FMSVer)
	props.AddProp("capabilities", r.Capabilities)
	enc.PutObject(&props)
	if err := r.Info.Encode(enc); err != nil {
		return err
	}
	return nil
}

func (r *ConnectCommandReply) Decode(dec *amf.Decoder) error {
	if err := r.cmdmesg.Header().Decode(dec); err != nil {
		return err
	}
	props, err := dec.GetObject()
	if err != nil {
		return fmt.Errorf("decode props: %w", err)
	}
	r.FMSVer = props.Prop("fmsVer").Value().String()
	r.Capabilities = int(props.Prop("capabilities").Value().Uint32())
	if err := r.Info.Decode(dec); err != nil {
		return fmt.Errorf("decode info: %w", err)
	}
	return nil
}

type CreateStreamCommandReply struct {
	cmdmesg
	Stream uint32
}

func (r *CreateStreamCommandReply) Encode(enc *amf.Encoder) error {
	if err := r.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	enc.PutNull()
	enc.PutUint32(r.Stream)
	return nil
}

func (r *CreateStreamCommandReply) Decode(dec *amf.Decoder) error {
	return nil
}

type PublishStreamCommandReply struct {
	cmdmesg
	Info CommandReplyInfo
}

func (r *PublishStreamCommandReply) Encode(enc *amf.Encoder) error {
	if err := r.cmdmesg.Header().Encode(enc); err != nil {
		return err
	}
	enc.PutNull()
	if err := r.Info.Encode(enc); err != nil {
		return err
	}
	return nil
}

func (r *PublishStreamCommandReply) Decode(dec *amf.Decoder) error {
	return nil
}
