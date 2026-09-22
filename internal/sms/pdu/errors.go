package pdu

import "errors"

// Sentinel errors for PDU encoding and decoding.
var (
	// ErrInvalidHex is returned when a PDU hex string contains invalid characters or odd length.
	ErrInvalidHex = errors.New("invalid hex string")

	// ErrTruncatedPDU is returned when the PDU byte stream ends unexpectedly.
	ErrTruncatedPDU = errors.New("truncated PDU")

	// ErrInvalidAddress is returned when a phone number is malformed.
	ErrInvalidAddress = errors.New("invalid phone number address")

	// ErrEmptyRecipient is returned when trying to encode an SMS without a recipient.
	ErrEmptyRecipient = errors.New("recipient address cannot be empty")

	// ErrEmptyText is returned when trying to encode an SMS without text content.
	ErrEmptyText = errors.New("message text cannot be empty")

	// ErrNotStatusReport is returned when attempting to decode a non-status-report PDU as status report.
	ErrNotStatusReport = errors.New("PDU is not an SMS-STATUS-REPORT")
)
