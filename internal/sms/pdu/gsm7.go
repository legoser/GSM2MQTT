package pdu

// gsm7BasicTable maps 7-bit values (0..127) to runes.
var gsm7BasicTable = [128]rune{
	'@', '£', '$', '¥', 'è', 'é', 'ù', 'ì', 'ò', 'Ç',
	'\n', 'Ø', 'ø', '\r', 'Å', 'å',
	'Δ', '_', 'Φ', 'Γ', 'Λ', 'Ω', 'Π', 'Ψ', 'Σ', 'Θ', 'Ξ',
	0, // 0x1B is ESC
	'Æ', 'æ', 'ß', 'É',
	' ', '!', '"', '#', '¤', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '<', '=', '>', '?',
	'¡', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', 'Ä', 'Ö', 'Ñ', 'Ü', '§',
	'¿', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'ä', 'ö', 'ñ', 'ü', 'à',
}

var gsm7ExtensionTable = map[byte]rune{
	0x0A: '\f',
	0x14: '^',
	0x28: '{',
	0x29: '}',
	0x2F: '\\',
	0x3C: '[',
	0x3D: '~',
	0x3E: ']',
	0x40: '|',
	0x65: '€',
}

var runeToGSM7Basic = func() map[rune]byte {
	m := make(map[rune]byte, 128)
	for i, r := range gsm7BasicTable {
		if i != 0x1B {
			m[r] = byte(i)
		}
	}
	return m
}()

var runeToGSM7Ext = func() map[rune]byte {
	m := make(map[rune]byte, len(gsm7ExtensionTable))
	for code, r := range gsm7ExtensionTable {
		m[r] = code
	}
	return m
}()

// IsGSM7 returns true if all characters in s belong to the GSM 7-bit default or extension alphabet.
func IsGSM7(s string) bool {
	for _, r := range s {
		if _, ok := runeToGSM7Basic[r]; !ok {
			if _, okExt := runeToGSM7Ext[r]; !okExt {
				return false
			}
		}
	}
	return true
}

// EncodeGSM7 converts a UTF-8 string to a sequence of 7-bit septets.
func EncodeGSM7(s string) []byte {
	var septets []byte
	for _, r := range s {
		if b, ok := runeToGSM7Basic[r]; ok {
			septets = append(septets, b)
		} else if ext, okExt := runeToGSM7Ext[r]; okExt {
			septets = append(septets, 0x1B, ext)
		} else {
			septets = append(septets, 0x3F) // '?'
		}
	}
	return septets
}

// PackSeptets packs 7-bit septets into 8-bit octets according to 3GPP TS 23.038.
// startBit is the bit offset in the first octet (0 for standard single SMS, or padding after UDH).
func PackSeptets(septets []byte, startBit int) []byte {
	if len(septets) == 0 {
		return nil
	}

	totalBits := startBit + len(septets)*7
	numOctets := (totalBits + 7) / 8
	octets := make([]byte, numOctets)

	bitPos := startBit
	for _, s := range septets {
		val := uint16(s & 0x7F)
		byteIdx := bitPos / 8
		bitOffset := bitPos % 8

		octets[byteIdx] |= byte(val << bitOffset)
		if byteIdx+1 < numOctets {
			octets[byteIdx+1] |= byte(val >> (8 - bitOffset))
		}
		bitPos += 7
	}

	return octets
}

// UnpackSeptets unpacks 8-bit octets into 7-bit septets.
func UnpackSeptets(octets []byte, numSeptets int, startBit int) []byte {
	septets := make([]byte, numSeptets)
	bitPos := startBit

	for i := 0; i < numSeptets; i++ {
		byteIdx := bitPos / 8
		bitOffset := bitPos % 8

		var val byte
		if byteIdx < len(octets) {
			val = octets[byteIdx] >> bitOffset
		}
		if bitOffset > 1 && byteIdx+1 < len(octets) {
			val |= octets[byteIdx+1] << (8 - bitOffset)
		}
		septets[i] = val & 0x7F
		bitPos += 7
	}

	return septets
}

// DecodeGSM7 decodes 7-bit septets into a UTF-8 string.
func DecodeGSM7(septets []byte) string {
	runes := make([]rune, 0, len(septets))
	for i := 0; i < len(septets); i++ {
		s := septets[i]
		if s == 0x1B && i+1 < len(septets) {
			i++
			if r, ok := gsm7ExtensionTable[septets[i]]; ok {
				runes = append(runes, r)
				continue
			}
		}
		if int(s) < len(gsm7BasicTable) {
			runes = append(runes, gsm7BasicTable[s])
		}
	}
	return string(runes)
}
