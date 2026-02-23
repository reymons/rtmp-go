package amf

import (
	"encoding/binary"
	"math"
)

type Decoder struct {
	ptr  int
	data []byte
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (c *Decoder) SetData(data []byte) {
	c.data = data
	c.ptr = 0
}

func (c *Decoder) SkipNull() {
	if c.ptr < len(c.data) && c.data[c.ptr] == markerNull {
		c.ptr += 1
	}
}

func (c *Decoder) GetString() (string, error) {
	if c.ptr+sizeStrAddr >= len(c.data) {
		return "", ErrBufferEmpty
	}

	marker := c.data[c.ptr]
	if marker != markerStr && marker != markerStrLong {
		return "", ErrIncorrectMarker
	}

	ptr := c.ptr + 1

	var strLen int

	if marker == markerStrLong {
		strLen = int(binary.BigEndian.Uint32(c.data[ptr : ptr+sizeStrLongAddr]))
		ptr += sizeStrLongAddr
	} else {
		strLen = int(binary.BigEndian.Uint16(c.data[ptr : ptr+sizeStrAddr]))
		ptr += sizeStrAddr
	}

	if ptr+strLen > len(c.data) {
		return "", ErrBufferEmpty
	}

	str := string(c.data[ptr : ptr+strLen])
	c.ptr = ptr + strLen

	return str, nil
}

func (c *Decoder) GetFloat64() (float64, error) {
	if c.ptr+sizeNum >= len(c.data) {
		return 0, ErrBufferEmpty
	}
	if c.data[c.ptr] != markerNum {
		return 0, ErrIncorrectMarker
	}

	ptr := c.ptr + sizeMarker
	n := math.Float64frombits(binary.BigEndian.Uint64(c.data[ptr:]))
	c.ptr = ptr + sizeNum

	return n, nil
}

func (c *Decoder) GetUint32() (uint32, error) {
	n, err := c.GetFloat64()
	return uint32(n), err
}

func (c *Decoder) GetBool() (bool, error) {
	if c.ptr+sizeBool >= len(c.data) {
		return false, ErrBufferEmpty
	}
	if c.data[c.ptr] != markerBool {
		return false, ErrIncorrectMarker
	}
	c.ptr += sizeMarker + sizeBool
	return c.data[c.ptr-sizeBool] != 0, nil
}

func (c *Decoder) GetObjectKey() (string, error) {
	if c.ptr+sizeObjKeyAddr >= len(c.data) {
		return "", ErrBufferEmpty
	}

	keyLen := int(binary.BigEndian.Uint16(c.data[c.ptr:]))

	if c.ptr+sizeObjKeyAddr+keyLen > len(c.data) {
		return "", ErrBufferEmpty
	}

	c.ptr += sizeObjKeyAddr
	key := string(c.data[c.ptr : c.ptr+keyLen])
	c.ptr += keyLen

	return key, nil
}

func (c *Decoder) GetObject() (Object, error) {
	if c.ptr >= len(c.data) {
		return Object{}, ErrBufferEmpty
	}

	marker := c.data[c.ptr]
	if marker != markerECMAArray && marker != markerObj {
		return Object{}, ErrIncorrectMarker
	}
	c.ptr += 1

	if marker == markerECMAArray {
		c.ptr += 4 // skip length, rely on end marker
	}

	obj := NewObject()

	for {
		key, err := c.GetObjectKey()
		if err != nil {
			return Object{}, err
		}

		if key == "" {
			if c.ptr < len(c.data) && c.data[c.ptr] == markerObjEnd {
				break
			}
			return Object{}, ErrIncorrectMarker
		}

		val, err := c.GetCurrent()
		if err != nil {
			return Object{}, err
		}

		obj.AddProp(key, val)
	}

	return obj, nil
}

func (c *Decoder) GetCurrent() (any, error) {
	if c.ptr >= len(c.data) {
		return nil, ErrBufferEmpty
	}

	switch c.data[c.ptr] {
	case markerNum:
		return c.GetFloat64()
	case markerStr, markerStrLong:
		return c.GetString()
	case markerBool:
		return c.GetBool()
	case markerObj, markerECMAArray:
		return c.GetObject()
	case markerNull:
		c.SkipNull()
		return nil, nil
	default:
		return nil, ErrInvalidMarker
	}
}
