package services

import (
	"context"
	"sync"
	"testing"
)

type mockCaller struct {
	mu         sync.Mutex
	dialed     string
	answered   bool
	hungup     bool
	dtmfDigits []string
	dialErr    error
	hangupErr  error
	answerErr  error
	dtmfErr    error
}

func (m *mockCaller) Dial(number string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dialed = number
	return m.dialErr
}

func (m *mockCaller) Answer() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answered = true
	return m.answerErr
}

func (m *mockCaller) Hangup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hungup = true
	return m.hangupErr
}

func (m *mockCaller) SendDTMF(digit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dtmfDigits = append(m.dtmfDigits, digit)
	return m.dtmfErr
}

func TestCallService_Dial(t *testing.T) {
	mock := &mockCaller{}
	svc := NewCallService("siemens_tc35", mock, nil)

	ctx := context.Background()
	err := svc.Dial(ctx, "+79991112233")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.dialed != "+79991112233" {
		t.Errorf("expected dialed number '+79991112233', got %q", mock.dialed)
	}
}

func TestCallService_Answer(t *testing.T) {
	mock := &mockCaller{}
	svc := NewCallService("siemens_tc35", mock, nil)

	ctx := context.Background()
	err := svc.Answer(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if !mock.answered {
		t.Errorf("expected answered = true")
	}
}

func TestCallService_Hangup(t *testing.T) {
	mock := &mockCaller{}
	svc := NewCallService("siemens_tc35", mock, nil)

	ctx := context.Background()
	err := svc.Hangup(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if !mock.hungup {
		t.Errorf("expected hungup = true")
	}
}

func TestCallService_SendDTMF(t *testing.T) {
	mock := &mockCaller{}
	svc := NewCallService("siemens_tc35", mock, nil)

	ctx := context.Background()
	err := svc.SendDTMF(ctx, "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.dtmfDigits) != 1 || mock.dtmfDigits[0] != "5" {
		t.Errorf("expected dtmf digit '5', got %v", mock.dtmfDigits)
	}
}

func TestCallService_IncomingCallURC(t *testing.T) {
	mock := &mockCaller{}
	var receivedEvent *CallEvent

	svc := NewCallService("siemens_tc35", mock, func(event CallEvent) {
		receivedEvent = &event
	})

	// Feed caller ID unsolicited result code
	svc.HandleURC(`+CLIP: "+79991112233",145,"",0,"",0`)

	if receivedEvent == nil {
		t.Fatal("expected incoming call event")
	}
	if receivedEvent.Type != "incoming" {
		t.Errorf("expected event type 'incoming', got %q", receivedEvent.Type)
	}
	if receivedEvent.From != "+79991112233" {
		t.Errorf("expected caller '+79991112233', got %q", receivedEvent.From)
	}
	if receivedEvent.ModemID != "siemens_tc35" {
		t.Errorf("expected ModemID 'siemens_tc35', got %q", receivedEvent.ModemID)
	}
}

func TestCallService_DTMF_URC(t *testing.T) {
	mock := &mockCaller{}
	var receivedEvent *CallEvent

	svc := NewCallService("siemens_tc35", mock, func(event CallEvent) {
		receivedEvent = &event
	})

	// Feed DTMF notification
	svc.HandleURC("+DTMF: 9")

	if receivedEvent == nil {
		t.Fatal("expected DTMF event")
	}
	if receivedEvent.Type != "dtmf" {
		t.Errorf("expected event type 'dtmf', got %q", receivedEvent.Type)
	}
	if receivedEvent.Digit != "9" {
		t.Errorf("expected digit '9', got %q", receivedEvent.Digit)
	}
}

func TestCallService_EndedURC(t *testing.T) {
	mock := &mockCaller{}
	var receivedEvent *CallEvent

	svc := NewCallService("siemens_tc35", mock, func(event CallEvent) {
		receivedEvent = &event
	})

	svc.HandleURC("NO CARRIER")

	if receivedEvent == nil {
		t.Fatal("expected ended call event")
	}
	if receivedEvent.Type != "ended" {
		t.Errorf("expected event type 'ended', got %q", receivedEvent.Type)
	}
	if receivedEvent.ModemID != "siemens_tc35" {
		t.Errorf("expected ModemID 'siemens_tc35', got %q", receivedEvent.ModemID)
	}
}

func TestCallService_CallDrop_AutoHangup_On_COLP(t *testing.T) {
	mock := &mockCaller{}
	var events []CallEvent
	svc := NewCallService("neoway_m590", mock, func(event CallEvent) {
		events = append(events, event)
	})

	// Initial status must be idle
	st := svc.Status()
	if st.State != CallStateIdle {
		t.Errorf("expected initial state 'idle', got %q", st.State)
	}

	ctx := context.Background()
	if err := svc.Dial(ctx, "+79964126670"); err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}

	// Status after dial should be ringing/dialing
	st = svc.Status()
	if st.State != CallStateRinging {
		t.Errorf("expected state 'ringing', got %q", st.State)
	}

	// Simulate incoming +COLP URC (call answered by recipient)
	svc.HandleURC(`+COLP: "+79964126670",145`)

	// Should trigger auto-hangup for call-drop
	mock.mu.Lock()
	hungup := mock.hungup
	mock.mu.Unlock()

	if !hungup {
		t.Errorf("expected auto-hangup on COLP answer")
	}

	st = svc.Status()
	if st.State != CallStateCompleted && st.State != CallStateAnswered {
		t.Errorf("expected state 'completed' or 'answered', got %q", st.State)
	}
}

func TestCallService_Status_EndedOnNoCarrier(t *testing.T) {
	mock := &mockCaller{}
	svc := NewCallService("neoway_m590", mock, nil)

	ctx := context.Background()
	_ = svc.Dial(ctx, "+79964126670")

	// URC NO CARRIER arrives
	svc.HandleURC("NO CARRIER")

	st := svc.Status()
	if st.State != CallStateCompleted {
		t.Errorf("expected state 'completed' after NO CARRIER, got %q", st.State)
	}
}

func TestCallService_Status_Busy(t *testing.T) {
	mock := &mockCaller{}
	svc := NewCallService("neoway_m590", mock, nil)

	ctx := context.Background()
	_ = svc.Dial(ctx, "+79964126670")

	// URC BUSY arrives
	svc.HandleURC("BUSY")

	st := svc.Status()
	if st.State != CallStateBusy {
		t.Errorf("expected state 'busy' after BUSY, got %q", st.State)
	}
}
