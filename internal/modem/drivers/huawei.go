package drivers

import (
	"context"
	"fmt"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// HuaweiDriver implements driver quirks for Huawei USB modems.
type HuaweiDriver struct {
	*BaseDriver
}

// NewHuaweiDriver creates a new HuaweiDriver.
func NewHuaweiDriver(runner ATRunner) *HuaweiDriver {
	return &HuaweiDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init initializes the Huawei USB modem.
func (d *HuaweiDriver) Init(ctx context.Context) error {
	if err := d.BaseDriver.Init(ctx); err != nil {
		return fmt.Errorf("huawei init failed: %w", err)
	}
	return nil
}

var _ modem.Driver = (*HuaweiDriver)(nil)
