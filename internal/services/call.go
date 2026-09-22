// Package services implements high-level orchestration services bridging modem and MQTT.
package services

import (
	"context"
	"strings"

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
	return s.caller.Dial(number)
}

// Answer answers an active incoming call.
func (s *CallService) Answer(ctx context.Context) error {
	return s.caller.Answer()
}

// Hangup terminates the current call.
func (s *CallService) Hangup(ctx context.Context) error {
	return s.caller.Hangup()
}

// SendDTMF transmits a DTMF tone during an active call.
func (s *CallService) SendDTMF(ctx context.Context, digit string) error {
	return s.caller.SendDTMF(digit)
}

// HandleURC processes incoming modem URC notifications related to voice calls (+CLIP, +DTMF, NO CARRIER, BUSY).
func (s *CallService) HandleURC(urc string) {
	if s.onEvent == nil {
		return
	}

	trimmed := strings.TrimSpace(urc)
	switch {
	case strings.HasPrefix(trimmed, "+CLIP:"):
		from := parseClipNumber(trimmed)
		s.onEvent(CallEvent{
			Type:    "incoming",
			From:    from,
			ModemID: s.modemID,
		})

	case strings.HasPrefix(trimmed, "+DTMF:"):
		digit := parseDTMFDigit(trimmed)
		s.onEvent(CallEvent{
			Type:    "dtmf",
			Digit:   digit,
			ModemID: s.modemID,
		})

	case trimmed == "NO CARRIER" || trimmed == "BUSY":
		s.onEvent(CallEvent{
			Type:    "ended",
			ModemID: s.modemID,
		})
	}
}

func parseClipNumber(urc string) string {
	payload := strings.TrimPrefix(urc, "+CLIP:")
	payload = strings.TrimSpace(payload)

	firstQuote := strings.Index(payload, "\"")
	if firstQuote != -1 {
		secondQuote := strings.Index(payload[firstQuote+1:], "\"")
		if secondQuote != -1 {
			return payload[firstQuote+1 : firstQuote+1+secondQuote]
		}
	}

	parts := strings.Split(payload, ",")
	if len(parts) > 0 {
		return strings.Trim(parts[0], "\" ")
	}
	return ""
}

func parseDTMFDigit(urc string) string {
	payload := strings.TrimPrefix(urc, "+DTMF:")
	payload = strings.TrimSpace(payload)
	return strings.Trim(payload, "\" ")
}
