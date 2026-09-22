package services

import (
	"context"
	"fmt"
	"sync"
)

// GatewayManager coordinates multiple ModemRunner instances and provides an aggregate facade.
type GatewayManager struct {
	mu      sync.RWMutex
	runners map[string]*ModemRunner
	order   []string
}

// NewGatewayManager creates a new GatewayManager.
func NewGatewayManager() *GatewayManager {
	return &GatewayManager{
		runners: make(map[string]*ModemRunner),
	}
}

// Register adds a ModemRunner to the manager.
func (m *GatewayManager) Register(runner *ModemRunner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runners[runner.mCfg.ID] = runner
	m.order = append(m.order, runner.mCfg.ID)
}

// GetModems returns the summaries of all registered modems.
func (m *GatewayManager) GetModems() []ModemSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]ModemSummary, 0, len(m.runners))
	for _, id := range m.order {
		if r, ok := m.runners[id]; ok {
			summaries = append(summaries, r.Summary())
		}
	}
	return summaries
}

// SendSMS dispatches an SMS through the requested (or first available) modem.
func (m *GatewayManager) SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error) {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return nil, err
	}
	return runner.SendSMS(ctx, to, text)
}

// SendUSSD dispatches a USSD query through the requested (or first available) modem.
func (m *GatewayManager) SendUSSD(ctx context.Context, modemID, code string) (string, error) {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return "", err
	}
	return runner.SendUSSD(ctx, code)
}

func (m *GatewayManager) findRunner(modemID string) (*ModemRunner, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modemID != "" {
		if r, ok := m.runners[modemID]; ok {
			return r, nil
		}
		return nil, fmt.Errorf("modem %q not found", modemID)
	}

	if len(m.order) > 0 {
		return m.runners[m.order[0]], nil
	}
	return nil, fmt.Errorf("no modems available")
}
