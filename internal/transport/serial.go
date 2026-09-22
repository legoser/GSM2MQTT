// Package transport provides serial port abstraction for modem communication.
package transport

import "io"

// Port represents a serial port connection.
// This interface abstracts the underlying serial library for testability.
type Port interface {
	io.ReadWriteCloser
}

// PortConfig holds serial port configuration parameters.
type PortConfig struct {
	Device      string
	BaudRate    int
	DataBits    int
	StopBits    int
	Parity      string
	FlowControl string
}

// Opener opens a serial port with the given configuration.
// This interface allows mocking serial port creation in tests.
type Opener interface {
	Open(cfg PortConfig) (Port, error)
}
