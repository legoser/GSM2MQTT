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
	cfg            StatusServiceConfig
	provider       modem.StatusProvider
	onSignal       func(rssi int, dbm int)
	onHealth       func(health ModemHealth)
	onDisconnect   func()
	failCount      int
	lastOperator   string
	lastRegistered bool
	lastTechnology string
	lastRoaming    bool
	lastRSSI       int
	lastSIM        modem.SIMState
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
	reg, errReg := s.provider.NetworkRegistration()
	op, _ := s.provider.OperatorName()
	sim, errSim := s.provider.SIMStatus()

	if op != "" {
		s.lastOperator = op
	} else if s.lastOperator != "" {
		op = s.lastOperator
	}

	s.updateFailCount(errRssi, errSim)
	rssi, sim = s.resolveSignalAndSIM(errRssi, rssi, errSim, sim)
	registered, networkStr := s.resolveRegistration(errReg, reg)

	dbm := calculateDBm(rssi)
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

func (s *StatusService) updateFailCount(errRssi, errSim error) {
	if errRssi != nil && errSim != nil {
		s.failCount++
		if s.failCount >= 3 && s.onDisconnect != nil {
			s.onDisconnect()
		}
	} else {
		s.failCount = 0
	}
}

func (s *StatusService) resolveSignalAndSIM(errRssi error, rssi int, errSim error, sim modem.SIMState) (int, modem.SIMState) {
	if errRssi == nil && rssi > 0 && rssi != 99 {
		s.lastRSSI = rssi
	} else if errRssi != nil && s.lastRSSI > 0 {
		rssi = s.lastRSSI
	}

	if errSim == nil && sim != "" {
		s.lastSIM = sim
	} else if errSim != nil && s.lastSIM != "" {
		sim = s.lastSIM
	}
	return rssi, sim
}

func (s *StatusService) resolveRegistration(errReg error, reg *modem.NetworkStatus) (bool, string) {
	networkStr := "unknown"
	registered := false
	if reg != nil && errReg == nil {
		registered = reg.Registered
		networkStr = reg.Technology
		if reg.Roaming {
			networkStr += " (roaming)"
		}
		s.lastRegistered = registered
		s.lastTechnology = reg.Technology
		s.lastRoaming = reg.Roaming
	} else if errReg != nil && s.lastRegistered {
		registered = s.lastRegistered
		networkStr = s.lastTechnology
		if s.lastRoaming {
			networkStr += " (roaming)"
		}
	}
	return registered, networkStr
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
	health, _ := s.Poll(ctx)
	if health != nil && (health.Status == "not_ready" || health.Status == "degraded") {
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
			_, _ = s.Poll(ctx)
		}
	}

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
