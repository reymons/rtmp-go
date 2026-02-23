package amf

import (
	"encoding/binary"
	"math"
)

type Encoder struct {
	data []byte
}

func NewEncoder() *Encoder {
	return &Encoder{
		data: make([]byte, 0, 200),
	}
}

func (c *Encoder) Reset() {
	c.data = c.data[:0]
}

func (c *Encoder) Data() []byte {
	return c.data
}

func (c *Encoder) setMarker(m uint8) {
	c.data = append(c.data, m)
}

func (c *Encoder) PutString(s string) {
	c.setMarker(markerStr)

	strLen := len(s)
	if strLen > math.MaxUint16 {
		c.data = binary.BigEndian.AppendUint32(c.data, uint32(strLen))
	} else {
		c.data = binary.BigEndian.AppendUint16(c.data, uint16(strLen))
	}

	c.data = append(c.data, s...)
}

func (c *Encoder) PutNull() {
	c.setMarker(markerNull)
}

func (c *Encoder) PutFloat64(n float64) {
	c.setMarker(markerNum)
	c.data = binary.BigEndian.AppendUint64(c.data, math.Float64bits(n))
}

func (c *Encoder) PutFloat32(n float32) {
	c.PutFloat64(float64(n))
}

func (c *Encoder) PutUint32(n uint32) {
	c.PutFloat64(float64(n))
}

func (c *Encoder) PutUint16(n uint16) {
	c.PutFloat64(float64(n))
}

func (c *Encoder) PutUint8(n uint8) {
	c.PutFloat64(float64(n))
}

func (c *Encoder) PutBool(flag bool) {
	c.setMarker(markerBool)

	var b byte
	if flag {
		b = 1
	}

	c.data = append(c.data, b)
}

func (c *Encoder) PutObject(obj *Object) error {
	c.setMarker(markerObj)

	var itr ObjectIterator
	obj.InitIterator(&itr)

	for {
		key, prop, ok := itr.Next()
		if !ok {
			break
		}

		if err := c.AddObjectKey(key); err != nil {
			return err
		}

		if err := c.Put(prop.Value().Get()); err != nil {
			return err
		}
	}

	c.data = append(c.data, 0, 0)
	c.setMarker(markerObjEnd)

	return nil
}

func (c *Encoder) Put(val any) error {
	var err error

	switch v := val.(type) {
	case float64:
		c.PutFloat64(v)
	case int:
		c.PutFloat64(float64(v))
	case uint:
		c.PutFloat64(float64(v))
	case int32:
		c.PutFloat64(float64(v))
	case int16:
		c.PutFloat64(float64(v))
	case int8:
		c.PutFloat64(float64(v))
	case uint32:
		c.PutUint32(v)
	case uint16:
		c.PutUint16(v)
	case uint8:
		c.PutUint8(v)
	case string:
		c.PutString(v)
	case bool:
		c.PutBool(v)
	case Object:
		c.PutObject(&v)
	case nil:
		c.PutNull()
	default:
		err = ErrUnsupportedDataType
	}

	return err
}

func (c *Encoder) AddObjectKey(key string) error {
	keyLen := len(key)

	if keyLen > math.MaxUint16 {
		return ErrObjKeyTooLong
	}

	c.data = binary.BigEndian.AppendUint16(c.data, uint16(keyLen))
	c.data = append(c.data, key...)

	return nil
}
