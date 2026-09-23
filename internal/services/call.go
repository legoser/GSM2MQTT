// Package services implements high-level orchestration services bridging modem and MQTT.
package services

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// CallState represents the current lifecycle state of a voice call.
type CallState string

const (
	CallStateIdle      CallState = "idle"
	CallStateDialing   CallState = "dialing"
	CallStateRinging   CallState = "ringing"
	CallStateAnswered  CallState = "answered"
	CallStateCompleted CallState = "completed"
	CallStateBusy      CallState = "busy"
	CallStateFailed    CallState = "failed"
)

// CallStatus contains real-time state and details about ongoing or recent voice call.
type CallStatus struct {
	State     CallState `json:"state"`
	Number    string    `json:"number,omitempty"`
	Direction string    `json:"direction,omitempty"` // "outgoing" or "incoming"
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

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
	mu      sync.RWMutex
	status  CallStatus
}

// NewCallService creates a new voice call service.
func NewCallService(modemID string, caller modem.Caller, onEvent func(event CallEvent)) *CallService {
	return &CallService{
		modemID: modemID,
		caller:  caller,
		onEvent: onEvent,
		status: CallStatus{
			State:   CallStateIdle,
			Message: "Idle",
		},
	}
}

// Status returns the current call state and details.
func (s *CallService) Status() CallStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Dial initiates an outgoing voice call and updates status to ringing.
func (s *CallService) Dial(ctx context.Context, number string) error {
	s.mu.Lock()
	s.status = CallStatus{
		State:     CallStateDialing,
		Number:    number,
		Direction: "outgoing",
		Message:   "Dialing " + number + "...",
		StartedAt: time.Now(),
	}
	s.mu.Unlock()

	slog.Info("modem dialing voice call", slog.String("modem", s.modemID), slog.String("number", number))
	if err := s.caller.Dial(number); err != nil {
		s.mu.Lock()
		s.status.State = CallStateFailed
		s.status.Message = err.Error()
		s.status.EndedAt = time.Now()
		s.mu.Unlock()

		slog.Error("modem voice call dial failed", slog.String("modem", s.modemID), slog.String("number", number), slog.Any("error", err))
		return err
	}

	s.mu.Lock()
	s.status.State = CallStateRinging
	s.status.Message = "Ringing " + number + "..."
	s.mu.Unlock()

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
	s.mu.Lock()
	s.status.State = CallStateAnswered
	s.status.Message = "Call active"
	s.mu.Unlock()

	slog.Info("modem voice call answered", slog.String("modem", s.modemID))
	return nil
}

// Hangup terminates the current call and marks status completed.
func (s *CallService) Hangup(ctx context.Context) error {
	slog.Info("modem terminating voice call", slog.String("modem", s.modemID))
	err := s.caller.Hangup()

	s.mu.Lock()
	s.status.State = CallStateCompleted
	s.status.Message = "Call terminated"
	s.status.EndedAt = time.Now()
	s.mu.Unlock()

	if err != nil {
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

// HandleURC processes incoming modem URC notifications related to voice calls.
func (s *CallService) HandleURC(urc string) {
	trimmed := strings.TrimSpace(urc)
	switch {
	case strings.HasPrefix(trimmed, "+CLIP:"):
		from := parseClipNumber(trimmed)
		s.mu.Lock()
		s.status = CallStatus{
			State:     CallStateRinging,
			Number:    from,
			Direction: "incoming",
			Message:   "Incoming call from " + from,
			StartedAt: time.Now(),
		}
		s.mu.Unlock()

		slog.Info("modem incoming call ringing", slog.String("modem", s.modemID), slog.String("from", from))
		if s.onEvent != nil {
			s.onEvent(CallEvent{
				Type:    "incoming",
				From:    from,
				ModemID: s.modemID,
			})
		}

	case strings.HasPrefix(trimmed, "+COLP:"):
		slog.Info("modem connected line (+COLP) received: call answered, initiating auto-hangup (call-drop)", slog.String("modem", s.modemID))
		s.mu.Lock()
		s.status.State = CallStateAnswered
		s.status.Message = "Answered! Auto-hanging up (call-drop)..."
		s.mu.Unlock()

		if s.onEvent != nil {
			s.onEvent(CallEvent{Type: "answered", ModemID: s.modemID})
		}

		_ = s.caller.Hangup()

		s.mu.Lock()
		s.status.State = CallStateCompleted
		s.status.Message = "Call answered and dropped successfully"
		s.status.EndedAt = time.Now()
		s.mu.Unlock()

		if s.onEvent != nil {
			s.onEvent(CallEvent{Type: "ended", ModemID: s.modemID})
		}

	case strings.HasPrefix(trimmed, "+DTMF:"):
		digit := parseDTMFDigit(trimmed)
		if s.onEvent != nil {
			s.onEvent(CallEvent{
				Type:    "dtmf",
				Digit:   digit,
				ModemID: s.modemID,
			})
		}

	case trimmed == "NO CARRIER":
		slog.Info("modem call ended event", slog.String("modem", s.modemID), slog.String("event", trimmed))
		s.mu.Lock()
		s.status.State = CallStateCompleted
		s.status.Message = "Call completed (NO CARRIER)"
		s.status.EndedAt = time.Now()
		s.mu.Unlock()

		if s.onEvent != nil {
			s.onEvent(CallEvent{
				Type:    "ended",
				ModemID: s.modemID,
			})
		}

	case trimmed == "BUSY":
		slog.Info("modem call busy event", slog.String("modem", s.modemID), slog.String("event", trimmed))
		s.mu.Lock()
		s.status.State = CallStateBusy
		s.status.Message = "Line busy / Rejected (BUSY)"
		s.status.EndedAt = time.Now()
		s.mu.Unlock()

		if s.onEvent != nil {
			s.onEvent(CallEvent{
				Type:    "ended",
				ModemID: s.modemID,
			})
		}
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
