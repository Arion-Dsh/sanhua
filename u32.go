package sanhua

func u32Bytes(u uint32) []byte {
	b := make([]byte, 4)
	b[0] = byte(0xff & (u >> 0))
	b[1] = byte(0xff & (u >> 8))
	b[2] = byte(0xff & (u >> 16))
	b[3] = byte(0xff & (u >> 24))
	return b
}

func bytesU32(b []byte) uint32 {
	_ = b[3]
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
