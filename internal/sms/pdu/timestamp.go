package pdu

import (
	"time"
)

// DecodeTimestamp decodes a 7-octet semi-octet BCD GSM timestamp into time.Time.
// Format: Year, Month, Day, Hour, Minute, Second, Timezone.
func DecodeTimestamp(data []byte) time.Time {
	if len(data) < 7 {
		return time.Now()
	}

	decodeBCD := func(b byte) int {
		return int(b&0x0F)*10 + int((b>>4)&0x0F)
	}

	year := 2000 + decodeBCD(data[0])
	month := time.Month(decodeBCD(data[1]))
	day := decodeBCD(data[2])
	hour := decodeBCD(data[3])
	minute := decodeBCD(data[4])
	second := decodeBCD(data[5])

	// Timezone is in quarters of an hour, bit 3 of data[6] indicates negative if set.
	tzRaw := data[6]
	tzQuarters := int(tzRaw&0x07)*10 + int((tzRaw>>4)&0x0F)
	if (tzRaw & 0x08) != 0 {
		tzQuarters = -tzQuarters
	}
	tzOffset := tzQuarters * 15 * 60

	loc := time.FixedZone("GSM", tzOffset)
	return time.Date(year, month, day, hour, minute, second, 0, loc)
}
