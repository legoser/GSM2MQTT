package pdu

import (
	"unicode/utf16"
)

// EncodeUCS2 encodes a UTF-8 string into big-endian UCS-2 / UTF-16 bytes.
func EncodeUCS2(s string) []byte {
	u16s := utf16.Encode([]rune(s))
	bytes := make([]byte, len(u16s)*2)
	for i, v := range u16s {
		bytes[i*2] = byte(v >> 8)
		bytes[i*2+1] = byte(v & 0xFF)
	}
	return bytes
}

// DecodeUCS2 decodes big-endian UCS-2 / UTF-16 bytes into a UTF-8 string.
func DecodeUCS2(bytes []byte) string {
	if len(bytes)%2 != 0 {
		bytes = bytes[:len(bytes)-1]
	}
	u16s := make([]uint16, len(bytes)/2)
	for i := 0; i < len(u16s); i++ {
		u16s[i] = (uint16(bytes[i*2]) << 8) | uint16(bytes[i*2+1])
	}
	return string(utf16.Decode(u16s))
}
