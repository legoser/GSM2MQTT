package modem

import (
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/transport"
)

// Engine is an alias for the AT engine for use by orchestrators.
type Engine = at.Engine

// Connector handles establishing transport connections to the modem.
// It depends only on transport.PortConfig (no config package import)
// so the modem layer stays decoupled from configuration structures.
type Connector struct {
	opener transport.Opener
}

// NewConnector creates a new modem connector with the given opener.
// Pass transport.NewSerialOpener() in production, a mock in tests.
func NewConnector(opener transport.Opener) *Connector {
	if opener == nil {
		opener = transport.NewSerialOpener()
	}
	return &Connector{opener: opener}
}

// Open creates a new AT Engine connected to the given port, and a closer function.
func (c *Connector) Open(cfg transport.PortConfig) (*Engine, func() error, error) {
	port, err := c.opener.Open(cfg)
	if err != nil {
		return nil, nil, err
	}
	return at.NewEngine(port), port.Close, nil
}
