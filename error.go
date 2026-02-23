package rtmp

import "errors"

var (
	errBufNoSpace = errors.New("buffer doesn't have enough space")
)
