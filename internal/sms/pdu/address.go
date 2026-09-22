package pdu

import (
	"fmt"
	"strings"
)

// EncodeAddress encodes an E.164 or national phone number into semi-octet BCD format for TP-DA.
// Returns length (number of digits), type of address byte (0x91 for intl, 0x81 for natl), and BCD bytes.
func EncodeAddress(number string) (digitCount byte, typeOfAddr byte, bcd []byte) {
	clean := strings.TrimSpace(number)
	typeOfAddr = 0x81 // National / unknown by default
	if strings.HasPrefix(clean, "+") {
		typeOfAddr = 0x91 // International format
		clean = clean[1:]
	}

	// Filter only digits
	var digits []byte
	for _, r := range clean {
		if r >= '0' && r <= '9' {
			digits = append(digits, byte(r-'0'))
		}
	}

	digitCount = byte(len(digits))
	if len(digits)%2 != 0 {
		digits = append(digits, 0x0F) // Pad trailing nibble
	}

	bcd = make([]byte, len(digits)/2)
	for i := 0; i < len(digits); i += 2 {
		bcd[i/2] = (digits[i+1] << 4) | digits[i]
	}

	return digitCount, typeOfAddr, bcd
}

// DecodeAddress decodes semi-octet BCD bytes into a phone number string.
// digitCount is the number of digits to read, typeOfAddr is 0x91 for intl.
func DecodeAddress(bcd []byte, digitCount int, typeOfAddr byte) string {
	var b strings.Builder
	if typeOfAddr == 0x91 {
		b.WriteByte('+')
	}

	digitsRead := 0
	for _, octet := range bcd {
		low := octet & 0x0F
		high := (octet >> 4) & 0x0F

		if low <= 9 && digitsRead < digitCount {
			b.WriteByte('0' + low)
			digitsRead++
		}
		if high <= 9 && digitsRead < digitCount {
			b.WriteByte('0' + high)
			digitsRead++
		}
	}

	return b.String()
}

// FormatAddressHex returns the hex string for TP-DA: <digitCount><typeOfAddr><bcdHex>.
func FormatAddressHex(number string) string {
	digits, toa, bcd := EncodeAddress(number)
	var hexParts strings.Builder
	hexParts.WriteString(fmt.Sprintf("%02X%02X", digits, toa))
	for _, b := range bcd {
		hexParts.WriteString(fmt.Sprintf("%02X", b))
	}
	return hexParts.String()
}
