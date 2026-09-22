package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/ussd"
)

// USSDSender transmits raw USSD requests to the modem.
type USSDSender interface {
	SendUSSD(code string) (string, error)
}

// USSDService coordinates USSD code dispatch and async URC reception.
type USSDService struct {
	modemID    string
	sender     USSDSender
	onResponse func(resp *ussd.Response)
	mu         sync.Mutex
	pendingCh  chan *ussd.Response
}

// NewUSSDService creates a new USSD service.
func NewUSSDService(modemID string, sender USSDSender, onResponse func(resp *ussd.Response)) *USSDService {
	return &USSDService{
		modemID:    modemID,
		sender:     sender,
		onResponse: onResponse,
	}
}

// Send validates the USSD code, submits it to the modem, and waits for a response.
func (s *USSDService) Send(ctx context.Context, code string) (*ussd.Response, error) {
	if err := ussd.ValidateCode(code); err != nil {
		return nil, err
	}

	respCh := make(chan *ussd.Response, 1)

	s.mu.Lock()
	s.pendingCh = respCh
	s.mu.Unlock()

	out, err := s.sender.SendUSSD(code)
	if err != nil {
		s.clearPending()
		return nil, err
	}

	if strings.Contains(out, "+CUSD:") {
		s.clearPending()
		return parseInlineCUSD(out)
	}

	timeout := 15 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		s.clearPending()
		return nil, ctx.Err()
	case <-time.After(timeout):
		s.clearPending()
		return nil, ussd.ErrUSSDTimeout
	}
}

// HandleURC parses incoming +CUSD indications and notifies listeners.
func (s *USSDService) HandleURC(urc string) {
	trimmed := strings.TrimSpace(urc)
	if !strings.HasPrefix(trimmed, "+CUSD:") {
		return
	}

	resp, err := ussd.ParseResponse(trimmed)
	if err != nil {
		return
	}

	s.mu.Lock()
	ch := s.pendingCh
	s.pendingCh = nil
	s.mu.Unlock()

	if ch != nil {
		select {
		case ch <- resp:
		default:
		}
	}

	if s.onResponse != nil {
		s.onResponse(resp)
	}
}

func (s *USSDService) clearPending() {
	s.mu.Lock()
	s.pendingCh = nil
	s.mu.Unlock()
}

func parseInlineCUSD(text string) (*ussd.Response, error) {
	idx := strings.Index(text, "+CUSD:")
	line := text[idx:]
	if end := strings.IndexByte(line, '\n'); end != -1 {
		line = line[:end]
	}
	return ussd.ParseResponse(strings.TrimSpace(line))
}
