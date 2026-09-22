package services

import (
	"context"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// StatusServiceConfig configures modem status monitoring.
type StatusServiceConfig struct {
	ModemID  string
	Interval time.Duration
}

// ModemHealth represents the consolidated operational health of a GSM modem.
type ModemHealth struct {
	Status    string `json:"status"` // ready, degraded, not_ready, error
	Signal    int    `json:"signal"`
	SignalDBm int    `json:"signal_dbm"`
	Operator  string `json:"operator,omitempty"`
	Network   string `json:"network,omitempty"`
	SIM       string `json:"sim"`
}

// StatusService polls modem state and publishes telemetry.
type StatusService struct {
	cfg          StatusServiceConfig
	provider     modem.StatusProvider
	onSignal     func(rssi int, dbm int)
	onHealth     func(health ModemHealth)
	onDisconnect func()
	failCount    int
}

// NewStatusService constructs a new StatusService.
func NewStatusService(
	cfg StatusServiceConfig,
	provider modem.StatusProvider,
	onSignal func(rssi int, dbm int),
	onHealth func(health ModemHealth),
) *StatusService {
	return &StatusService{
		cfg:      cfg,
		provider: provider,
		onSignal: onSignal,
		onHealth: onHealth,
	}
}

// SetOnDisconnect sets a callback triggered when communication fails repeatedly.
func (s *StatusService) SetOnDisconnect(fn func()) {
	s.onDisconnect = fn
}

// Poll queries modem status and updates listeners.
func (s *StatusService) Poll(ctx context.Context) (*ModemHealth, error) {
	rssi, errRssi := s.provider.SignalQuality()
	reg, _ := s.provider.NetworkRegistration()
	op, _ := s.provider.OperatorName()
	sim, errSim := s.provider.SIMStatus()

	if errRssi != nil && errSim != nil {
		s.failCount++
		if s.failCount >= 3 && s.onDisconnect != nil {
			s.onDisconnect()
		}
	} else {
		s.failCount = 0
	}

	dbm := calculateDBm(rssi)

	networkStr := "unknown"
	registered := false
	if reg != nil {
		registered = reg.Registered
		networkStr = reg.Technology
		if reg.Roaming {
			networkStr += " (roaming)"
		}
	}

	overallStatus := determineStatus(sim, registered, rssi)

	health := &ModemHealth{
		Status:    overallStatus,
		Signal:    rssi,
		SignalDBm: dbm,
		Operator:  op,
		Network:   networkStr,
		SIM:       string(sim),
	}

	if s.onSignal != nil && rssi > 0 && rssi != 99 {
		s.onSignal(rssi, dbm)
	}
	if s.onHealth != nil {
		s.onHealth(*health)
	}

	return health, nil
}

// Start launches a periodic polling background loop.
func (s *StatusService) Start(ctx context.Context) {
	interval := s.cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial poll
	_, _ = s.Poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.Poll(ctx)
		}
	}
}

func calculateDBm(rssi int) int {
	if rssi == 0 {
		return -113
	}
	if rssi >= 1 && rssi <= 31 {
		return -113 + (2 * rssi)
	}
	return -999
}

func determineStatus(sim modem.SIMState, registered bool, rssi int) string {
	if sim != modem.SIMReady && sim != "" {
		return "not_ready"
	}
	if !registered || rssi < 5 {
		return "degraded"
	}
	return "ready"
}
