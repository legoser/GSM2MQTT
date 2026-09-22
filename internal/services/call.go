// Package services implements high-level orchestration services bridging modem and MQTT.
package services

import (
	"context"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// CallEvent represents an incoming or state-changed call event.
type CallEvent struct {
	Type    string `json:"type"` // incoming, answered, ended, dtmf
	From    string `json:"from,omitempty"`
	Digit   string `json:"digit,omitempty"`
	ModemID string `json:"modem_id"`
}

// CallService coordinates voice call interactions with the modem.
type CallService struct {
	caller  modem.Caller
	modemID string
	onEvent func(event CallEvent)
}

// NewCallService creates a new voice call service.
func NewCallService(modemID string, caller modem.Caller, onEvent func(event CallEvent)) *CallService {
	return &CallService{
		modemID: modemID,
		caller:  caller,
		onEvent: onEvent,
	}
}

// Dial initiates an outgoing voice call.
func (s *CallService) Dial(ctx context.Context, number string) error {
	// STUB for TDD: will fail tests
	return nil
}

// Answer answers an active incoming call.
func (s *CallService) Answer(ctx context.Context) error {
	// STUB for TDD: will fail tests
	return nil
}

// Hangup terminates the current call.
func (s *CallService) Hangup(ctx context.Context) error {
	// STUB for TDD: will fail tests
	return nil
}

// SendDTMF transmits a DTMF tone during an active call.
func (s *CallService) SendDTMF(ctx context.Context, digit string) error {
	// STUB for TDD: will fail tests
	return nil
}

// HandleURC processes incoming modem URC notifications related to voice calls (+CLIP, +DTMF).
func (s *CallService) HandleURC(urc string) {
	// STUB for TDD: will fail tests
}
