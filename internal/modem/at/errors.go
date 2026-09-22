// Package at implements the AT command communication engine,
// command serialization, timeout management, and URC routing.
package at

import "errors"

var (
	// ErrTimeout is returned when an AT command does not receive a final response within timeout.
	ErrTimeout = errors.New("at command timed out")

	// ErrPortClosed is returned when the underlying I/O stream is closed.
	ErrPortClosed = errors.New("serial port is closed")

	// ErrCommandFailed is returned when the modem responds with ERROR or CMS/CME error.
	ErrCommandFailed = errors.New("at command returned error response")
)
