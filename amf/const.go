package amf

import (
	"errors"
)

const (
	markerNum       uint8 = 0x00
	markerBool            = 0x01
	markerStr             = 0x02
	markerStrLong         = 0x0c
	markerObj             = 0x03
	markerECMAArray       = 0x08
	markerObjEnd          = 0x09
	markerNull            = 0x05
)

const (
	sizeMarker      = 1
	sizeNum         = 8
	sizeBool        = 1
	sizeStrAddr     = 2
	sizeStrLongAddr = 4
	sizeObjKeyAddr  = 2
	sizeObjEnd      = 2
	sizeNull        = 1
)

var (
	ErrBufferEmpty         = errors.New("buffer is empty")
	ErrObjKeyTooLong       = errors.New("object key is too long, should fit into 16 bits")
	ErrIncorrectMarker     = errors.New("incorrect marker")
	ErrNumOutOfRange       = errors.New("number out of range")
	ErrInvalidMarker       = errors.New("invalid marker")
	ErrUnsupportedDataType = errors.New("data type is unsupported")
	ErrNoObjProp           = errors.New("no object property with such a key")
)
