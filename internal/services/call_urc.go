package services

import (
	"log/slog"
	"strings"
	"time"
)

// parseClipNumber extracts the caller phone number from a +CLIP URC string.
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

// parseDTMFDigit extracts the decoded DTMF character from a +DTMF URC string.
func parseDTMFDigit(urc string) string {
	payload := strings.TrimPrefix(urc, "+DTMF:")
	payload = strings.TrimSpace(payload)
	return strings.Trim(payload, "\" ")
}

func (s *CallService) handleClipURC(urc string) {
	from := parseClipNumber(urc)
	s.mu.Lock()
	s.stopDropTimer()
	startedAt := time.Now()
	if s.status.State == CallStateRinging && !s.status.StartedAt.IsZero() {
		startedAt = s.status.StartedAt
	}
	s.status = CallStatus{
		State:     CallStateRinging,
		Number:    from,
		Direction: "incoming",
		Message:   "Incoming call from " + from,
		StartedAt: startedAt,
	}
	s.mu.Unlock()

	slog.Info("modem incoming call ringing", slog.String("modem", s.modemID), slog.String("from", from))
	if s.onEvent != nil {
		s.onEvent(CallEvent{
			Type:      "incoming",
			From:      from,
			Number:    from,
			Direction: "incoming",
			ModemID:   s.modemID,
		})
	}
}

func (s *CallService) handleColpURC(urc string) {
	s.handleCallAnsweredDrop()
}

func (s *CallService) handleDTMFURC(urc string) {
	digit := parseDTMFDigit(urc)
	if s.onEvent != nil {
		s.onEvent(CallEvent{
			Type:    "dtmf",
			Digit:   digit,
			ModemID: s.modemID,
		})
	}
}

func (s *CallService) handleNoCarrierURC() {
	slog.Info("modem call ended event", slog.String("modem", s.modemID), slog.String("event", "NO CARRIER"))
	s.mu.Lock()
	s.stopDropTimer()
	wasAnswered := s.status.State == CallStateAnswered
	var duration time.Duration
	if wasAnswered && !s.status.AnsweredAt.IsZero() {
		duration = time.Since(s.status.AnsweredAt)
	}
	num := s.status.Number
	dir := s.status.Direction
	statusStr := "completed"
	if wasAnswered {
		s.status.State = CallStateCompleted
		s.status.Message = "Call finished"
		s.addLogLocked("Call finished (NO CARRIER)")
	} else {
		if dir == "incoming" {
			statusStr = "missed"
		} else {
			statusStr = "failed"
		}
		s.status.State = CallStateFailed
		s.status.Message = "Call ended (no answer / disconnected)"
		s.addLogLocked("Call ended: NO CARRIER (no answer / disconnected)")
	}
	s.status.EndedAt = time.Now()
	s.mu.Unlock()

	if s.onEvent != nil {
		s.onEvent(CallEvent{
			Type:      "ended",
			ModemID:   s.modemID,
			Duration:  duration,
			Number:    num,
			From:      num,
			Direction: dir,
			Status:    statusStr,
		})
	}
}

func (s *CallService) handleBusyURC() {
	slog.Info("modem call busy event", slog.String("modem", s.modemID), slog.String("event", "BUSY"))
	s.mu.Lock()
	s.stopDropTimer()
	num := s.status.Number
	dir := s.status.Direction
	statusStr := "busy"
	if dir == "incoming" {
		statusStr = "rejected"
	}
	s.status.State = CallStateBusy
	s.status.Message = "Line busy / Rejected"
	s.status.EndedAt = time.Now()
	s.addLogLocked("Line busy / Call rejected (BUSY)")
	s.mu.Unlock()

	if s.onEvent != nil {
		s.onEvent(CallEvent{
			Type:      "ended",
			ModemID:   s.modemID,
			Number:    num,
			From:      num,
			Direction: dir,
			Status:    statusStr,
		})
	}
}

func (s *CallService) monitorCall(checker CallStateChecker, stopCh <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			s.mu.RLock()
			st := s.status.State
			s.mu.RUnlock()

			if st != CallStateDialing && st != CallStateRinging {
				return
			}

			callState, err := checker.CheckCallState()
			if err != nil {
				continue
			}

			switch callState {
			case "ringing":
				s.mu.Lock()
				if s.status.State == CallStateDialing {
					s.status.State = CallStateRinging
					s.status.Message = "Ringing " + s.status.Number + "..."
					s.addLogLocked("Remote party is ringing (alerting)...")
				}
				s.mu.Unlock()
			case "answered":
				s.handleCallAnsweredDrop()
				return
			case "idle":
				return
			}
		}
	}
}

func (s *CallService) handleCallAnsweredDrop() {
	slog.Info("call answered by recipient, initiating auto-hangup (call-drop)", slog.String("modem", s.modemID))
	s.mu.Lock()
	s.stopDropTimer()
	s.status.State = CallStateAnswered
	s.status.AnsweredAt = time.Now()
	s.status.Message = "Answered! Auto-hanging up (call-drop)..."
	num := s.status.Number
	dir := s.status.Direction
	s.mu.Unlock()

	if s.onEvent != nil {
		s.onEvent(CallEvent{
			Type:      "answered",
			ModemID:   s.modemID,
			Number:    num,
			From:      num,
			Direction: dir,
		})
	}

	_ = s.caller.Hangup()

	s.mu.Lock()
	duration := time.Since(s.status.AnsweredAt)
	s.status.State = CallStateCompleted
	s.status.Message = "Call answered and dropped successfully"
	s.status.EndedAt = time.Now()
	s.addLogLocked("Call dropped successfully (call completed)")
	s.mu.Unlock()

	if s.onEvent != nil {
		s.onEvent(CallEvent{
			Type:      "ended",
			ModemID:   s.modemID,
			Duration:  duration,
			Number:    num,
			From:      num,
			Direction: dir,
			Status:    "completed",
		})
	}
}
