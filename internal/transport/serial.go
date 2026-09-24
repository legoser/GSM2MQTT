// Package transport provides serial port abstraction for modem communication.
package transport

import (
	"fmt"
	"io"
	"strings"
	"time"

	"go.bug.st/serial"
)

// Port represents a serial port connection with flow and modem control.
type Port interface {
	io.ReadWriteCloser
	SetDTR(dtr bool) error
	SetRTS(rts bool) error
	ResetInputBuffer() error
	ResetOutputBuffer() error
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

// Validate checks whether the port configuration contains valid values.
func (c *PortConfig) Validate() error {
	if strings.TrimSpace(c.Device) == "" {
		return ErrEmptyDevice
	}
	if c.BaudRate <= 0 {
		return ErrInvalidBaudRate
	}

	dataBits := c.DataBits
	if dataBits == 0 {
		dataBits = 8
	}
	if dataBits < 5 || dataBits > 8 {
		return ErrInvalidDataBits
	}

	if c.StopBits != 0 && c.StopBits != 1 && c.StopBits != 2 && c.StopBits != 15 {
		return ErrInvalidStopBits
	}

	if err := validateParity(c.Parity); err != nil {
		return err
	}

	return validateFlowControl(c.FlowControl)
}

// ToSerialMode converts PortConfig into go.bug.st/serial.Mode.
func (c *PortConfig) ToSerialMode() (*serial.Mode, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	dataBits := c.DataBits
	if dataBits == 0 {
		dataBits = 8
	}

	stopBits := serial.OneStopBit
	switch c.StopBits {
	case 2:
		stopBits = serial.TwoStopBits
	case 15:
		stopBits = serial.OnePointFiveStopBits
	}

	parity, _ := parseParity(c.Parity)

	return &serial.Mode{
		BaudRate: c.BaudRate,
		DataBits: dataBits,
		StopBits: stopBits,
		Parity:   parity,
	}, nil
}

// Opener opens a serial port with the given configuration.
type Opener interface {
	Open(cfg PortConfig) (Port, error)
}

// SerialOpener opens real serial ports using go.bug.st/serial.
type SerialOpener struct{}

// NewSerialOpener creates a new default SerialOpener.
func NewSerialOpener() *SerialOpener {
	return &SerialOpener{}
}

// Open validates configuration and opens the physical serial port.
func (o *SerialOpener) Open(cfg PortConfig) (Port, error) {
	mode, err := cfg.ToSerialMode()
	if err != nil {
		return nil, fmt.Errorf("invalid port configuration: %w", err)
	}

	p, err := serial.Open(cfg.Device, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", cfg.Device, err)
	}

	if err := p.SetReadTimeout(1 * time.Second); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("failed to set read timeout on %s: %w", cfg.Device, err)
	}

	return p, nil
}

func validateParity(p string) error {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "", "none", "odd", "even", "mark", "space":
		return nil
	default:
		return ErrInvalidParity
	}
}

func parseParity(p string) (serial.Parity, error) {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "", "none":
		return serial.NoParity, nil
	case "odd":
		return serial.OddParity, nil
	case "even":
		return serial.EvenParity, nil
	case "mark":
		return serial.MarkParity, nil
	case "space":
		return serial.SpaceParity, nil
	default:
		return serial.NoParity, ErrInvalidParity
	}
}

func validateFlowControl(fc string) error {
	fc = strings.ToLower(strings.TrimSpace(fc))
	switch fc {
	case "", "none", "hardware", "software":
		return nil
	default:
		return ErrInvalidFlowControl
	}
}
