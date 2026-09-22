package pdu

import "time"

// DecodedSMS represents a decoded incoming SMS (SMS-DELIVER).
type DecodedSMS struct {
	From       string
	Text       string
	Timestamp  time.Time
	Encoding   Encoding
	HasUDH     bool
	Reference  byte
	PartNumber int
	TotalParts int
}

// DecodeSMS decodes a hex-encoded SMS-DELIVER PDU string.
// Unimplemented stub for TDD.
func DecodeSMS(pduHex string) (*DecodedSMS, error) {
	// STUB for TDD: will fail tests
	return nil, nil
}
