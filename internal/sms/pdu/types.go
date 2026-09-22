// Package pdu implements 3GPP TS 23.040 and 23.038 PDU encoding and decoding.
package pdu

import "time"

// Encoding represents the character encoding scheme for SMS.
type Encoding string

const (
	EncodingAuto   Encoding = "auto"
	EncodingGSM7   Encoding = "gsm7"
	EncodingUCS2   Encoding = "ucs2"
	EncodingBinary Encoding = "binary"
)

// PDU represents a formatted Protocol Data Unit for AT+CMGS.
type PDU struct {
	// CommandLength is the TPDU length (octets) required by AT+CMGS=<length>.
	CommandLength int
	// Hex is the hexadecimal string sent to the modem.
	Hex string
	// HasUDH indicates if User Data Header is present.
	HasUDH bool
	// Encoding used for payload.
	Encoding Encoding
	// PartNumber for multipart SMS (1-based), or 1 for single SMS.
	PartNumber int
	// TotalParts for multipart SMS, or 1 for single SMS.
	TotalParts int
}

// StatusReport represents a parsed SMS-STATUS-REPORT (+CDS).
type StatusReport struct {
	MessageRef  byte
	Recipient   string
	SCTimestamp time.Time
	Discharge   time.Time
	StatusCode  byte
	Delivered   bool
	Temporary   bool
	Permanent   bool
}
