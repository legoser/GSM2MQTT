// Package services implements high-level orchestration services bridging modem and MQTT.
package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// DefaultAutoDropTimeout is the default duration to allow an outgoing call to ring before auto-hanging up.
const DefaultAutoDropTimeout = 60 * time.Second

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

// CallStateChecker queries active call status from the modem (e.g. via AT+CLCC).
type CallStateChecker interface {
	CheckCallState() (string, error)
}

// CallLogEntry records a timestamped lifecycle event in a voice call.
type CallLogEntry struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

// CallStatus contains real-time state and details about ongoing or recent voice call.
type CallStatus struct {
	State     CallState      `json:"state"`
	Number    string         `json:"number,omitempty"`
	Direction string         `json:"direction,omitempty"` // "outgoing" or "incoming"
	Message   string         `json:"message"`
	StartedAt time.Time      `json:"started_at,omitempty"`
	EndedAt   time.Time      `json:"ended_at,omitempty"`
	Logs      []CallLogEntry `json:"logs,omitempty"`
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
	monitorStop     chan struct{}
	monitorInterval time.Duration
}

// NewCallService creates a new voice call service.
func NewCallService(modemID string, caller modem.Caller, onEvent func(event CallEvent)) *CallService {
	return &CallService{
		modemID:         modemID,
		caller:          caller,
		onEvent:         onEvent,
		autoDropTimeout: DefaultAutoDropTimeout,
		monitorInterval: 1 * time.Second,
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

func (s *CallService) addLogLocked(msg string) {
	s.status.Logs = append(s.status.Logs, CallLogEntry{
		Time:    time.Now(),
		Message: msg,
	})
	if len(s.status.Logs) > 20 {
		s.status.Logs = s.status.Logs[len(s.status.Logs)-20:]
	}
}

func (s *CallService) stopDropTimer() {
	if s.dropTimer != nil {
		s.dropTimer.Stop()
		s.dropTimer = nil
	}
	if s.monitorStop != nil {
		close(s.monitorStop)
		s.monitorStop = nil
	}
}

func (s *CallService) onAutoDropTimeout() {
	s.mu.Lock()
	if s.status.State != CallStateDialing && s.status.State != CallStateRinging {
		s.mu.Unlock()
		return
	}
	slog.Info("call-drop timeout reached, terminating call", slog.String("modem", s.modemID))
	s.status.State = CallStateFailed
	s.status.Message = "Call timed out (no answer)"
	s.status.EndedAt = time.Now()
	s.addLogLocked("Call timed out (no answer)")
	if s.monitorStop != nil {
		close(s.monitorStop)
		s.monitorStop = nil
	}
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

func cleanVoiceNumber(raw string) string {
	trimmed := strings.TrimSpace(raw)
	var b strings.Builder
	for i, r := range trimmed {
		if r == '+' && i == 0 {
			b.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Dial initiates an outgoing voice call, sanitizes number, and sets auto-drop timer and monitor.
func (s *CallService) Dial(ctx context.Context, number string) error {
	clean := cleanVoiceNumber(number)
	if clean == "" {
		return errors.New("empty phone number")
	}

	s.mu.Lock()
	s.stopDropTimer()
	s.status = CallStatus{
		State:     CallStateDialing,
		Number:    clean,
		Direction: "outgoing",
		Message:   "Dialing " + clean + "...",
		StartedAt: time.Now(),
		Logs: []CallLogEntry{
			{Time: time.Now(), Message: "Initiating outgoing call to " + clean + "..."},
		},
	}
	s.mu.Unlock()

	slog.Info("modem dialing voice call", slog.String("modem", s.modemID), slog.String("number", clean))
	if err := s.caller.Dial(clean); err != nil {
		s.mu.Lock()
		s.status.State = CallStateFailed
		s.status.Message = err.Error()
		s.status.EndedAt = time.Now()
		s.addLogLocked("Dial failed: " + err.Error())
		s.mu.Unlock()

		slog.Error("modem voice call dial failed", slog.String("modem", s.modemID), slog.String("number", clean), slog.Any("error", err))
		return err
	}

	s.startDialMonitoring(clean)
	slog.Info("modem voice call dial command accepted", slog.String("modem", s.modemID), slog.String("number", clean))
	return nil
}

func (s *CallService) startDialMonitoring(clean string) {
	s.mu.Lock()
	s.status.State = CallStateRinging
	s.status.Message = "Ringing " + clean + "..."
	s.addLogLocked("Modem accepted dial command, connecting network...")
	timeout := s.autoDropTimeout
	if timeout <= 0 {
		timeout = DefaultAutoDropTimeout
	}
	s.dropTimer = time.AfterFunc(timeout, s.onAutoDropTimeout)
	stopCh := make(chan struct{})
	s.monitorStop = stopCh
	interval := s.monitorInterval
	if interval <= 0 {
		interval = 1 * time.Second
	}
	s.mu.Unlock()

	if checker, ok := s.caller.(CallStateChecker); ok {
		go s.monitorCall(checker, stopCh, interval)
	}
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
	s.addLogLocked("Call answered (active)")
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
	s.addLogLocked("Call terminated by user")
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
	slog.Info("modem transmitting DTMF tone", slog.String("modem", s.modemID), slog.String("digit", digit))
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
