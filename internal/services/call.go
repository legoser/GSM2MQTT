// Package services implements high-level orchestration services bridging modem and MQTT.
package services

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/sms"
)

// DefaultAutoDropTimeout is the default duration to allow an outgoing call to ring before auto-hanging up.
const DefaultAutoDropTimeout = 25 * time.Second

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
	caller          modem.Caller
	modemID         string
	onEvent         func(event CallEvent)
	mu              sync.RWMutex
	status          CallStatus
	autoDropTimeout time.Duration
	dropTimer       *time.Timer
}

// NewCallService creates a new voice call service.
func NewCallService(modemID string, caller modem.Caller, onEvent func(event CallEvent)) *CallService {
	return &CallService{
		modemID:         modemID,
		caller:          caller,
		onEvent:         onEvent,
		autoDropTimeout: DefaultAutoDropTimeout,
		status: CallStatus{
			State:   CallStateIdle,
			Message: "Idle",
		},
	}
}

// SetAutoDropTimeout configures the timeout for auto-dropping outgoing calls.
func (s *CallService) SetAutoDropTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoDropTimeout = d
}

func (s *CallService) stopDropTimer() {
	if s.dropTimer != nil {
		s.dropTimer.Stop()
		s.dropTimer = nil
	}
}

func (s *CallService) onAutoDropTimeout() {
	s.mu.Lock()
	if s.status.State != CallStateDialing && s.status.State != CallStateRinging {
		s.mu.Unlock()
		return
	}
	slog.Info("call-drop timeout reached, terminating call", slog.String("modem", s.modemID))
	s.status.State = CallStateCompleted
	s.status.Message = "Call completed (auto-drop timeout reached)"
	s.status.EndedAt = time.Now()
	s.mu.Unlock()

	_ = s.caller.Hangup()
	if s.onEvent != nil {
		s.onEvent(CallEvent{Type: "ended", ModemID: s.modemID})
	}
}

// Status returns the current call state and details.
func (s *CallService) Status() CallStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Dial initiates an outgoing voice call, normalizes number, and sets auto-drop timer.
func (s *CallService) Dial(ctx context.Context, number string) error {
	norm := strings.TrimSpace(number)
	if n, err := sms.NormalizePhoneNumber(norm, "+7"); err == nil {
		norm = n
	}

	s.mu.Lock()
	s.stopDropTimer()
	s.status = CallStatus{
		State:     CallStateDialing,
		Number:    norm,
		Direction: "outgoing",
		Message:   "Dialing " + norm + "...",
		StartedAt: time.Now(),
	}
	s.mu.Unlock()

	slog.Info("modem dialing voice call", slog.String("modem", s.modemID), slog.String("number", norm))
	if err := s.caller.Dial(norm); err != nil {
		s.mu.Lock()
		s.status.State = CallStateFailed
		s.status.Message = err.Error()
		s.status.EndedAt = time.Now()
		s.mu.Unlock()

		slog.Error("modem voice call dial failed", slog.String("modem", s.modemID), slog.String("number", norm), slog.Any("error", err))
		return err
	}

	s.mu.Lock()
	s.status.State = CallStateRinging
	s.status.Message = "Ringing " + norm + "..."
	timeout := s.autoDropTimeout
	if timeout <= 0 {
		timeout = DefaultAutoDropTimeout
	}
	s.dropTimer = time.AfterFunc(timeout, s.onAutoDropTimeout)
	s.mu.Unlock()

	slog.Info("modem voice call dial command accepted", slog.String("modem", s.modemID), slog.String("number", norm))
	return nil
}

// Answer answers an active incoming call.
func (s *CallService) Answer(ctx context.Context) error {
	slog.Info("modem answering voice call", slog.String("modem", s.modemID))
	s.mu.Lock()
	s.stopDropTimer()
	s.mu.Unlock()

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
	s.mu.Lock()
	s.stopDropTimer()
	s.status.State = CallStateCompleted
	s.status.Message = "Call terminated"
	s.status.EndedAt = time.Now()
	s.mu.Unlock()

	err := s.caller.Hangup()
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
		s.handleClipURC(trimmed)
	case strings.HasPrefix(trimmed, "+COLP:"):
		s.handleColpURC(trimmed)
	case strings.HasPrefix(trimmed, "+DTMF:"):
		s.handleDTMFURC(trimmed)
	case trimmed == "NO CARRIER":
		s.handleNoCarrierURC()
	case trimmed == "BUSY":
		s.handleBusyURC()
	}
}
