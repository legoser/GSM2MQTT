package drivers

import (
	"context"
	"fmt"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// SiemensDriver implements driver quirks for Siemens/Cinterion TC35, MC55, TC65 modems.
type SiemensDriver struct {
	*BaseDriver
}

// NewSiemensDriver creates a new SiemensDriver.
func NewSiemensDriver(runner ATRunner) *SiemensDriver {
	return &SiemensDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init synchronizes the auto-baud rate on RS-232 and executes initialization.
func (d *SiemensDriver) Init(ctx context.Context) error {
	// Ping AT to synchronize baud rate
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := d.runner.Send("AT", 1*time.Second)
		if err == nil && resp.OK {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := d.BaseDriver.Init(ctx); err != nil {
		return fmt.Errorf("siemens init failed: %w", err)
	}

	return nil
}

var _ modem.Driver = (*SiemensDriver)(nil)
