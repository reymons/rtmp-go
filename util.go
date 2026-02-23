package rtmp

func reverseMap[K comparable, V comparable](m map[K]V) map[V]K {
	result := make(map[V]K)
	for k, v := range m {
		result[v] = k
	}
	return result
}

func encode3BytesBE(buf []byte, n uint32) {
	if len(buf) < 3 {
		panic("expected a buffer length at least 3 byte long")
	}

	buf[0] = byte(n >> 16)
	buf[1] = byte(n >> 8)
	buf[2] = byte(n)
}

func decode3BytesBE(b []byte) uint32 {
	if len(b) < 3 {
		panic("size should be at least 3 bytes")
	}

	return uint32(b[0])<<16 |
		uint32(b[1])<<8 |
		uint32(b[2])
}
