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
}

func (s *CallService) handleColpURC(urc string) {
	slog.Info("modem connected line (+COLP) received: call answered, initiating auto-hangup (call-drop)", slog.String("modem", s.modemID))
	s.mu.Lock()
	s.stopDropTimer()
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
}

func (s *CallService) handleBusyURC() {
	slog.Info("modem call busy event", slog.String("modem", s.modemID), slog.String("event", "BUSY"))
	s.mu.Lock()
	s.stopDropTimer()
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
