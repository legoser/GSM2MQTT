package security

// MaskPhone masks the middle part of a phone number for safe logging.
// It keeps the first 5 characters (e.g. +7999) and the last 4 characters.
func MaskPhone(number string) string {
	if len(number) < 6 {
		return "***"
	}
	start := 5
	if len(number) > 0 && number[0] != '+' {
		start = 4
	}
	end := len(number) - 4
	if start >= end {
		return number[:2] + "***" + number[len(number)-2:]
	}
	return number[:start] + "***" + number[end:]
}
