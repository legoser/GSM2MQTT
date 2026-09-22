package transport

import (
	"errors"
	"testing"
)

func TestPortConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PortConfig
		wantErr error
	}{
		{
			name: "valid standard config",
			cfg: PortConfig{
				Device:      "/dev/ttyUSB0",
				BaudRate:    115200,
				DataBits:    8,
				StopBits:    1,
				Parity:      "none",
				FlowControl: "none",
			},
			wantErr: nil,
		},
		{
			name: "valid hardware flow control",
			cfg: PortConfig{
				Device:      "/dev/ttyS0",
				BaudRate:    9600,
				DataBits:    8,
				StopBits:    1,
				Parity:      "none",
				FlowControl: "hardware",
			},
			wantErr: nil,
		},
		{
			name: "valid even parity",
			cfg: PortConfig{
				Device:      "/dev/ttyUSB1",
				BaudRate:    19200,
				DataBits:    7,
				StopBits:    2,
				Parity:      "even",
				FlowControl: "none",
			},
			wantErr: nil,
		},
		{
			name: "empty device",
			cfg: PortConfig{
				Device:   "",
				BaudRate: 115200,
			},
			wantErr: ErrEmptyDevice,
		},
		{
			name: "invalid baud rate zero",
			cfg: PortConfig{
				Device:   "/dev/ttyUSB0",
				BaudRate: 0,
			},
			wantErr: ErrInvalidBaudRate,
		},
		{
			name: "invalid data bits",
			cfg: PortConfig{
				Device:   "/dev/ttyUSB0",
				BaudRate: 115200,
				DataBits: 4,
			},
			wantErr: ErrInvalidDataBits,
		},
		{
			name: "invalid stop bits",
			cfg: PortConfig{
				Device:   "/dev/ttyUSB0",
				BaudRate: 115200,
				DataBits: 8,
				StopBits: 3,
			},
			wantErr: ErrInvalidStopBits,
		},
		{
			name: "invalid parity",
			cfg: PortConfig{
				Device:   "/dev/ttyUSB0",
				BaudRate: 115200,
				DataBits: 8,
				StopBits: 1,
				Parity:   "invalid_parity",
			},
			wantErr: ErrInvalidParity,
		},
		{
			name: "invalid flow control",
			cfg: PortConfig{
				Device:      "/dev/ttyUSB0",
				BaudRate:    115200,
				DataBits:    8,
				StopBits:    1,
				Parity:      "none",
				FlowControl: "invalid_flow",
			},
			wantErr: ErrInvalidFlowControl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestSerialOpener_NonExistentPort(t *testing.T) {
	opener := NewSerialOpener()
	cfg := PortConfig{
		Device:      "/dev/nonexistent_gsm_modem_port",
		BaudRate:    115200,
		DataBits:    8,
		StopBits:    1,
		Parity:      "none",
		FlowControl: "none",
	}

	_, err := opener.Open(cfg)
	if err == nil {
		t.Errorf("expected error opening non-existent device, got nil")
	}
}
