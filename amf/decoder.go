package amf

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Decoder struct {
	ptr  int
	data []byte
}

func NewDecoder(data []byte) Decoder {
	return Decoder{data: data}
}

func (c *Decoder) SkipNull() {
	if c.ptr < len(c.data) && c.data[c.ptr] == MarkerNull {
		c.ptr += 1
	}
}

func (c *Decoder) GetString() (string, error) {
	if c.ptr+sizeStrAddr >= len(c.data) {
		return "", ErrBufferEmpty
	}

	marker := c.data[c.ptr]
	if marker != MarkerStr && marker != MarkerStrLong {
		return "", ErrIncorrectMarker
	}

	ptr := c.ptr + 1

	var strLen int

	if marker == MarkerStrLong {
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
	if c.data[c.ptr] != MarkerNum {
		return 0, ErrIncorrectMarker
	}

	ptr := c.ptr + sizeMarker
	n := math.Float64frombits(binary.BigEndian.Uint64(c.data[ptr:]))
	c.ptr = ptr + sizeNum

	return n, nil
}

func (c *Decoder) GetBool() (bool, error) {
	if c.ptr+sizeBool >= len(c.data) {
		return false, ErrBufferEmpty
	}
	if c.data[c.ptr] != MarkerBool {
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
		fmt.Printf("%d\n", keyLen)
		return "", ErrBufferEmpty
	}

	c.ptr += sizeObjKeyAddr
	key := string(c.data[c.ptr : c.ptr+keyLen])
	c.ptr += keyLen

	return key, nil
}

func (c *Decoder) GetObject() (Object, error) {
	if c.ptr >= len(c.data) {
		return nil, ErrBufferEmpty
	}

	marker := c.data[c.ptr]
	if marker != MarkerECMAArray && marker != MarkerObj {
		return nil, ErrIncorrectMarker
	}
	c.ptr += 1

	if marker == MarkerECMAArray {
		c.ptr += 4 // skip length, rely on end marker
	}

	obj := NewObject()

	for {
		key, err := c.GetObjectKey()
		if err != nil {
			return nil, err
		}

		if key == "" {
			if c.ptr < len(c.data) && c.data[c.ptr] == MarkerObjEnd {
				break
			}
			return nil, ErrIncorrectMarker
		}

		val, err := c.GetCurrent()
		if err != nil {
			return nil, err
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
	case MarkerNum:
		return c.GetFloat64()
	case MarkerStr, MarkerStrLong:
		return c.GetString()
	case MarkerBool:
		return c.GetBool()
	case MarkerObj:
		return c.GetObject()
	default:
		return nil, ErrInvalidMarker
	}
}
