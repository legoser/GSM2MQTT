package modem

import (
	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/transport"
)

// Engine is an alias for the AT engine for use by orchestrators.
type Engine = at.Engine

// Connector handles establishing transport connections to the modem.
type Connector struct {
	opener transport.Opener
}

// NewConnector creates a new modem connector.
func NewConnector() *Connector {
	return &Connector{
		opener: transport.NewSerialOpener(),
	}
}

// Open creates a new AT Engine connected to the configured modem port, and a closer function.
func (c *Connector) Open(mCfg config.ModemConfig) (*Engine, func() error, error) {
	port, err := c.opener.Open(transport.PortConfig{
		Device:      mCfg.Port,
		BaudRate:    mCfg.BaudRate,
		DataBits:    mCfg.DataBits,
		StopBits:    mCfg.StopBits,
		Parity:      mCfg.Parity,
		FlowControl: mCfg.FlowControl,
	})
	if err != nil {
		return nil, nil, err
	}
	return at.NewEngine(port), port.Close, nil
}

// WithOpener is for testing
func (c *Connector) WithOpener(o transport.Opener) {
	c.opener = o
}
