package amf

import (
	"encoding/binary"
	"math"
)

type Encoder struct {
	data []byte
	ptr  int
}

func NewEncoder(data []byte) Encoder {
	return Encoder{data: data}
}

func (c *Encoder) GetData() []byte {
	return c.data[:c.ptr]
}

func (c *Encoder) setMarker(m uint8) {
	c.data[c.ptr] = m
	c.ptr += 1
}

func (c *Encoder) PutString(s string) error {
	strLen := len(s)

	if c.ptr+sizeMarker+strLen > len(c.data) {
		return ErrBufferFull
	}

	c.setMarker(MarkerStr)

	if strLen > math.MaxUint16 {
		binary.BigEndian.PutUint32(c.data[c.ptr:c.ptr+sizeStrLongAddr], uint32(strLen))
		c.ptr += sizeStrLongAddr
	} else {
		binary.BigEndian.PutUint16(c.data[c.ptr:c.ptr+sizeStrAddr], uint16(strLen))
		c.ptr += sizeStrAddr
	}

	copy(c.data[c.ptr:], s)
	c.ptr += strLen

	return nil
}

func (c *Encoder) PutNull() error {
	if c.ptr+sizeMarker+sizeNull > len(c.data) {
		return ErrBufferFull
	}

	c.setMarker(MarkerNull)
	return nil
}

func (c *Encoder) PutFloat64(n float64) error {
	if c.ptr+sizeMarker+sizeNum > len(c.data) {
		return ErrBufferFull
	}

	c.setMarker(MarkerNum)
	binary.BigEndian.PutUint64(c.data[c.ptr:c.ptr+sizeNum], math.Float64bits(n))
	c.ptr += sizeNum

	return nil
}

func (c *Encoder) PutFloat32(n float32) error {
	return c.PutFloat64(float64(n))
}

func (c *Encoder) PutUint32(n uint32) error {
	return c.PutFloat64(float64(n))
}

func (c *Encoder) PutUint16(n uint16) error {
	return c.PutFloat64(float64(n))
}

func (c *Encoder) PutUint8(n uint8) error {
	return c.PutFloat64(float64(n))
}

func (c *Encoder) PutBool(f bool) error {
	if c.ptr+sizeMarker+sizeBool > len(c.data) {
		return ErrBufferFull
	}

	c.setMarker(MarkerBool)

	if f {
		c.data[c.ptr+1] = 1
	} else {
		c.data[c.ptr+1] = 0
	}
	c.ptr += 1

	return nil
}

func (c *Encoder) PutObject(obj Object) error {
	if err := c.StartObject(); err != nil {
		return err
	}

	for key, prop := range obj {
		if err := c.AddObjectKey(key); err != nil {
			return err
		}

		if err := c.Put(prop.GetValue().Get()); err != nil {
			return err
		}
	}

	if err := c.EndObject(); err != nil {
		return err
	}

	return nil
}

func (c *Encoder) Put(val any) error {
	switch v := val.(type) {
	case float64:
		return c.PutFloat64(v)
	case int:
		return c.PutFloat64(float64(v))
	case uint:
		return c.PutFloat64(float64(v))
	case int32:
		return c.PutFloat64(float64(v))
	case int16:
		return c.PutFloat64(float64(v))
	case int8:
		return c.PutFloat64(float64(v))
	case uint32:
		return c.PutUint32(v)
	case uint16:
		return c.PutUint16(v)
	case uint8:
		return c.PutUint8(v)
	case string:
		return c.PutString(v)
	case bool:
		return c.PutBool(v)
	case Object:
		return c.PutObject(v)
	case nil:
		return c.PutNull()
	default:
		return ErrUnsupportedDataType
	}
}

func (c *Encoder) StartObject() error {
	if c.ptr+sizeMarker > len(c.data) {
		return ErrBufferFull
	}

	c.setMarker(MarkerObj)

	return nil
}

func (c *Encoder) EndObject() error {
	if c.ptr+sizeMarker+sizeObjEnd > len(c.data) {
		return ErrBufferFull
	}

	c.data[c.ptr] = 0
	c.data[c.ptr+1] = 0
	c.ptr += sizeObjEnd

	c.setMarker(MarkerObjEnd)

	return nil
}

func (c *Encoder) AddObjectKey(key string) error {
	keyLen := len(key)

	if keyLen > math.MaxUint16 {
		return ErrObjKeyTooLong
	}

	if c.ptr+sizeObjKeyAddr+keyLen > len(c.data) {
		return ErrBufferFull
	}

	binary.BigEndian.PutUint16(c.data[c.ptr:], uint16(keyLen))
	c.ptr += sizeObjKeyAddr

	copy(c.data[c.ptr:], key)
	c.ptr += keyLen

	return nil
}
