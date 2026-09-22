// Package ussd implements GSM USSD (Unstructured Supplementary Service Data) command execution,
// response parsing, DCS decoding, and error handling.
package ussd

import "errors"

// Sentinel errors for USSD operations.
var (
	// ErrEmptyUSSDCode is returned when the USSD request code is empty.
	ErrEmptyUSSDCode = errors.New("USSD code is empty")

	// ErrInvalidUSSDFormat is returned when the USSD code doesn't conform to USSD format (e.g. missing * or #).
	ErrInvalidUSSDFormat = errors.New("invalid USSD code format")

	// ErrUSSDTimeout is returned when the network times out or returns status 5.
	ErrUSSDTimeout = errors.New("USSD request timed out")

	// ErrUSSDTerminated is returned when the network abruptly terminates the session (status 2).
	ErrUSSDTerminated = errors.New("USSD session terminated by network")

	// ErrUSSDNotSupported is returned when the operator or modem does not support the USSD request (status 4).
	ErrUSSDNotSupported = errors.New("USSD operation not supported by network")

	// ErrMalformedUSSDResponse is returned when the +CUSD response or URC is corrupt or cannot be parsed.
	ErrMalformedUSSDResponse = errors.New("malformed +CUSD response")
)
