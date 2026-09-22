// Package transport provides serial port abstraction and hardware communication
// for GSM modems.
package transport

import "errors"

var (
	// ErrEmptyDevice is returned when the serial device path is empty.
	ErrEmptyDevice = errors.New("serial device path cannot be empty")

	// ErrInvalidBaudRate is returned when the baud rate is non-positive or unsupported.
	ErrInvalidBaudRate = errors.New("invalid baud rate")

	// ErrInvalidDataBits is returned when data bits is not 5, 6, 7, or 8.
	ErrInvalidDataBits = errors.New("invalid data bits")

	// ErrInvalidStopBits is returned when stop bits value is invalid.
	ErrInvalidStopBits = errors.New("invalid stop bits")

	// ErrInvalidParity is returned when parity is unknown.
	ErrInvalidParity = errors.New("invalid parity setting")

	// ErrInvalidFlowControl is returned when flow control mode is unknown.
	ErrInvalidFlowControl = errors.New("invalid flow control setting")
)
