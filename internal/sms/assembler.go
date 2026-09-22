package sms

import (
	"time"
)

// IncomingPart represents a single received SMS part that may belong to a multipart message.
type IncomingPart struct {
	From        string
	Text        string
	Timestamp   time.Time
	IsMultipart bool
	Reference   byte
	PartNumber  int
	TotalParts  int
	Encoding    string
}

// AssembledSMS represents a complete received SMS message.
type AssembledSMS struct {
	From      string
	Text      string
	Timestamp time.Time
	Segments  int
	Encoding  string
}

// Assembler handles reassembly of multipart SMS messages.
type Assembler struct {
	ttl time.Duration
}

// NewAssembler creates a new SMS Assembler with the given part expiration TTL.
func NewAssembler(ttl time.Duration) *Assembler {
	return &Assembler{ttl: ttl}
}

// AddPart processes an incoming SMS part.
// If the message is complete (or not multipart), it returns the assembled message and true.
// If more parts are expected, it returns nil and false.
func (a *Assembler) AddPart(part IncomingPart) (*AssembledSMS, bool) {
	// STUB for TDD: will fail tests
	return nil, false
}
