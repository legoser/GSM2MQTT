package drivers

import (
	"github.com/legoser/gsm2mqtt/internal/modem"
)

// GenericDriver represents a generic AT modem driver adhering to 3GPP standards.
type GenericDriver struct {
	*BaseDriver
}

// NewGenericDriver creates a new GenericDriver.
func NewGenericDriver(runner ATRunner) *GenericDriver {
	return &GenericDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

var _ modem.Driver = (*GenericDriver)(nil)
