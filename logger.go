//go:build debug

package rtmp

import "log"

func Debugf(format string, v ...any) {
	log.Printf("[DEBUG] "+format, v...)
}
