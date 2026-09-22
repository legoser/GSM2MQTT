package services

import (
	"context"
	"fmt"
)

// ID returns the configured identifier of the modem.
func (r *ModemRunner) ID() string {
	return r.mCfg.ID
}

// Status returns the operational status of the modem.
func (r *ModemRunner) Status() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.lastHealth.Status == "" {
		return "ready"
	}
	return r.lastHealth.Status
}

// Signal returns the CSQ signal strength RSSI.
func (r *ModemRunner) Signal() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastHealth.Signal
}

// Operator returns the detected network operator name.
func (r *ModemRunner) Operator() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastHealth.Operator
}

// Dial initiates a voice call on this modem.
func (r *ModemRunner) Dial(ctx context.Context, number string) error {
	r.mu.RLock()
	svc := r.callSvc
	r.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("call service not initialized")
	}
	return svc.Dial(ctx, number)
}

// Hangup terminates any active voice call on this modem.
func (r *ModemRunner) Hangup(ctx context.Context) error {
	r.mu.RLock()
	svc := r.callSvc
	r.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("call service not initialized")
	}
	return svc.Hangup(ctx)
}
