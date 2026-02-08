package amf

import (
	"errors"
)

const (
	MarkerNum       uint8 = 0x00
	MarkerBool            = 0x01
	MarkerStr             = 0x02
	MarkerStrLong         = 0x0c
	MarkerObj             = 0x03
	MarkerECMAArray       = 0x08
	MarkerObjEnd          = 0x09
	MarkerNull            = 0x05
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
	ErrBufferFull          = errors.New("buffer is full")
	ErrBufferEmpty         = errors.New("buffer is empty")
	ErrObjKeyTooLong       = errors.New("object key is too long, should fit into 16 bits")
	ErrIncorrectMarker     = errors.New("incorrect marker")
	ErrNumOutOfRange       = errors.New("number out of range")
	ErrInvalidMarker       = errors.New("invalid marker")
	ErrUnsupportedDataType = errors.New("data type is unsupported")
	ErrNoObjProp           = errors.New("no object property with such a key")
)
