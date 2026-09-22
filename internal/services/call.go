// Package services implements high-level orchestration services bridging modem and MQTT.
package services

import (
	"context"
	"log/slog"
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
	slog.Info("modem dialing voice call", slog.String("modem", s.modemID), slog.String("number", number))
	if err := s.caller.Dial(number); err != nil {
		slog.Error("modem voice call dial failed", slog.String("modem", s.modemID), slog.String("number", number), slog.Any("error", err))
		return err
	}
	slog.Info("modem voice call dial command accepted", slog.String("modem", s.modemID), slog.String("number", number))
	return nil
}

// Answer answers an active incoming call.
func (s *CallService) Answer(ctx context.Context) error {
	slog.Info("modem answering voice call", slog.String("modem", s.modemID))
	if err := s.caller.Answer(); err != nil {
		slog.Error("modem voice call answer failed", slog.String("modem", s.modemID), slog.Any("error", err))
		return err
	}
	slog.Info("modem voice call answered", slog.String("modem", s.modemID))
	return nil
}

// Hangup terminates the current call.
func (s *CallService) Hangup(ctx context.Context) error {
	slog.Info("modem terminating voice call", slog.String("modem", s.modemID))
	if err := s.caller.Hangup(); err != nil {
		slog.Error("modem voice call hangup failed", slog.String("modem", s.modemID), slog.Any("error", err))
		return err
	}
	slog.Info("modem voice call terminated", slog.String("modem", s.modemID))
	return nil
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
		slog.Info("modem incoming call ringing", slog.String("modem", s.modemID), slog.String("from", from))
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
		slog.Info("modem call ended event", slog.String("modem", s.modemID), slog.String("event", trimmed))
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
