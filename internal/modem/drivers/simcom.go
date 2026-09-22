package drivers

import (
	"context"
	"fmt"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// SIMComDriver implements driver quirks for SIMCom SIM800/SIM900 modems.
type SIMComDriver struct {
	*BaseDriver
}

// NewSIMComDriver creates a new SIMComDriver.
func NewSIMComDriver(runner ATRunner) *SIMComDriver {
	return &SIMComDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init configures sleep mode and initializes base parameters.
func (d *SIMComDriver) Init(ctx context.Context) error {
	// Disable sleep mode on UART
	_, _ = d.runner.Send("AT+CSCLK=0", 1*time.Second)

	if err := d.BaseDriver.Init(ctx); err != nil {
		return fmt.Errorf("simcom init failed: %w", err)
	}

	return nil
}

var _ modem.Driver = (*SIMComDriver)(nil)
