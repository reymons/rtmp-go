package rtmp

import (
	"github.com/reymons/amf-go"
)

const (
	DataMesgSetDataFrame = "@setDataFrame"
)

const (
	DataMesgEventMetaData = "onMetaData"
)

type DataMessage interface {
	Encode(enc *amf.Encoder) error

	Decode(dec *amf.Decoder) error
}

type MetaDataMessage struct {
	VideoCodecID    uint8
	VideoDataRate   uint16
	AudioChannels   uint8
	AudioCodecID    uint8
	AudioDataRate   uint16
	AudioSampleRate uint32
	AudioSampleSize uint8
	Encoder         string
	FrameRate       uint8
	Width           uint16
	Height          uint16
	Stereo          bool
}

func (m *MetaDataMessage) Encode(enc *amf.Encoder) error {
	enc.PutString(DataMesgSetDataFrame)
	enc.PutString(DataMesgEventMetaData)
	obj := amf.NewObject()
	obj.AddProp("videocodecid", m.VideoCodecID)
	obj.AddProp("videodatarate", m.VideoDataRate)
	obj.AddProp("audiochannels", m.AudioChannels)
	obj.AddProp("audiocodecid", m.AudioCodecID)
	obj.AddProp("audiodatarate", m.AudioDataRate)
	obj.AddProp("audiosamplerate", m.AudioSampleRate)
	obj.AddProp("audiosamplesize", m.AudioSampleSize)
	obj.AddProp("encoder", m.Encoder)
	obj.AddProp("framerate", m.FrameRate)
	obj.AddProp("width", m.Width)
	obj.AddProp("height", m.Height)
	obj.AddProp("stereo", m.Stereo)
	if err := enc.PutObject(&obj); err != nil {
		return err
	}
	return nil
}

func (m *MetaDataMessage) Decode(dec *amf.Decoder) error {
	obj, err := dec.GetObject()
	if err != nil {
		return err
	}
	m.VideoCodecID = obj.Prop("videocodecid").Value().Uint8()
	m.VideoDataRate = obj.Prop("videodatarate").Value().Uint16()
	m.AudioChannels = obj.Prop("audiochannels").Value().Uint8()
	m.AudioCodecID = obj.Prop("audiocodecid").Value().Uint8()
	m.AudioDataRate = obj.Prop("audiodatarate").Value().Uint16()
	m.AudioSampleRate = obj.Prop("audiosamplerate").Value().Uint32()
	m.AudioSampleSize = obj.Prop("audiosamplesize").Value().Uint8()
	m.Encoder = obj.Prop("encoder").Value().String()
	m.FrameRate = obj.Prop("framerate").Value().Uint8()
	m.Width = obj.Prop("width").Value().Uint16()
	m.Height = obj.Prop("height").Value().Uint16()
	m.Stereo = obj.Prop("stereo").Value().Bool()
	return nil
}
