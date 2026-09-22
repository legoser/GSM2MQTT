package pool

import (
	"context"
	"errors"
	"testing"
)

type mockMember struct {
	id          string
	status      string
	signal      int
	operator    string
	sendErr     error
	dialErr     error
	ussdErr     error
	sendCalls   int
	dialCalls   int
	ussdCalls   int
	lastTo      string
	lastText    string
	lastDialNum string
	lastUSSD    string
}

func (m *mockMember) ID() string       { return m.id }
func (m *mockMember) Status() string   { return m.status }
func (m *mockMember) Signal() int      { return m.signal }
func (m *mockMember) Operator() string { return m.operator }

func (m *mockMember) SendSMS(ctx context.Context, to, text string) ([]byte, error) {
	m.sendCalls++
	m.lastTo = to
	m.lastText = text
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	return []byte{1}, nil
}

func (m *mockMember) SendUSSD(ctx context.Context, code string) (string, error) {
	m.ussdCalls++
	m.lastUSSD = code
	if m.ussdErr != nil {
		return "", m.ussdErr
	}
	return "OK", nil
}

func (m *mockMember) Dial(ctx context.Context, number string) error {
	m.dialCalls++
	m.lastDialNum = number
	return m.dialErr
}

func (m *mockMember) Hangup(ctx context.Context) error {
	return nil
}

func TestPool_Strategy_RoundRobin(t *testing.T) {
	m1 := &mockMember{id: "m1", status: "ready"}
	m2 := &mockMember{id: "m2", status: "ready"}
	m3 := &mockMember{id: "m3", status: "error"} // Should be skipped

	p, err := New(Config{Strategy: StrategyRoundRobin})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	p.Register(m1)
	p.Register(m2)
	p.Register(m3)

	// Call 1 -> m1
	sel1, err := p.Select(context.Background(), SelectionCriteria{})
	if err != nil || sel1.ID() != "m1" {
		t.Errorf("call 1: expected m1, got %v, err=%v", sel1, err)
	}

	// Call 2 -> m2
	sel2, err := p.Select(context.Background(), SelectionCriteria{})
	if err != nil || sel2.ID() != "m2" {
		t.Errorf("call 2: expected m2, got %v, err=%v", sel2, err)
	}

	// Call 3 -> loops back to m1
	sel3, err := p.Select(context.Background(), SelectionCriteria{})
	if err != nil || sel3.ID() != "m1" {
		t.Errorf("call 3: expected m1, got %v, err=%v", sel3, err)
	}
}

func TestPool_Strategy_Failover(t *testing.T) {
	m1 := &mockMember{id: "m1", status: "not_ready"}
	m2 := &mockMember{id: "m2", status: "ready"}
	m3 := &mockMember{id: "m3", status: "ready"}

	p, err := New(Config{Strategy: StrategyFailover})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	p.Register(m1)
	p.Register(m2)
	p.Register(m3)

	sel, err := p.Select(context.Background(), SelectionCriteria{})
	if err != nil || sel.ID() != "m2" {
		t.Errorf("expected primary ready modem m2, got %v, err=%v", sel, err)
	}
}

func TestPool_Strategy_BestSignal(t *testing.T) {
	m1 := &mockMember{id: "m1", status: "ready", signal: 12}
	m2 := &mockMember{id: "m2", status: "ready", signal: 28}
	m3 := &mockMember{id: "m3", status: "ready", signal: 19}

	p, err := New(Config{Strategy: StrategyBestSignal})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	p.Register(m1)
	p.Register(m2)
	p.Register(m3)

	sel, err := p.Select(context.Background(), SelectionCriteria{})
	if err != nil || sel.ID() != "m2" {
		t.Errorf("expected best signal m2, got %v, err=%v", sel, err)
	}
}

func TestPool_Strategy_OperatorMatch(t *testing.T) {
	m1 := &mockMember{id: "m1", status: "ready", operator: "MegaFon"}
	m2 := &mockMember{id: "m2", status: "ready", operator: "MTS"}

	p, err := New(Config{Strategy: StrategyOperatorMatch})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	p.Register(m1)
	p.Register(m2)

	// Destination starting with +7910 (MTS range)
	sel, err := p.Select(context.Background(), SelectionCriteria{RecipientNumber: "+79101234567"})
	if err != nil || sel.ID() != "m2" {
		t.Errorf("expected MTS modem m2 for +7910, got %v, err=%v", sel, err)
	}

	// Destination starting with +7926 (MegaFon range)
	selMega, err := p.Select(context.Background(), SelectionCriteria{RecipientNumber: "+79261234567"})
	if err != nil || selMega.ID() != "m1" {
		t.Errorf("expected MegaFon modem m1 for +7926, got %v, err=%v", selMega, err)
	}
}

func TestPool_SendSMS_FailoverRetry(t *testing.T) {
	m1 := &mockMember{id: "m1", status: "ready", sendErr: errors.New("sim error")}
	m2 := &mockMember{id: "m2", status: "ready"}

	p, err := New(Config{Strategy: StrategyFailover})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	p.Register(m1)
	p.Register(m2)

	ref, err := p.SendSMS(context.Background(), "+79001112233", "Hello failover")
	if err != nil {
		t.Fatalf("expected successful failover send, got err: %v", err)
	}
	if len(ref) == 0 {
		t.Errorf("expected non-empty ref")
	}
	if m1.sendCalls != 1 || m2.sendCalls != 1 {
		t.Errorf("expected m1 and m2 to both be called, got m1=%d, m2=%d", m1.sendCalls, m2.sendCalls)
	}
}

func TestPool_NegativeCases(t *testing.T) {
	t.Run("empty pool returns ErrNoReadyModems", func(t *testing.T) {
		p, _ := New(Config{Strategy: StrategyRoundRobin})
		_, err := p.Select(context.Background(), SelectionCriteria{})
		if !errors.Is(err, ErrNoReadyModems) {
			t.Errorf("expected ErrNoReadyModems, got: %v", err)
		}
	})

	t.Run("all modems non-ready returns ErrNoReadyModems", func(t *testing.T) {
		m := &mockMember{id: "m1", status: "error"}
		p, _ := New(Config{Strategy: StrategyRoundRobin})
		p.Register(m)
		_, err := p.Select(context.Background(), SelectionCriteria{})
		if !errors.Is(err, ErrNoReadyModems) {
			t.Errorf("expected ErrNoReadyModems, got: %v", err)
		}
	})

	t.Run("all modems fail send returns ErrAllModemsFailed", func(t *testing.T) {
		m1 := &mockMember{id: "m1", status: "ready", sendErr: errors.New("fail 1")}
		m2 := &mockMember{id: "m2", status: "ready", sendErr: errors.New("fail 2")}
		p, _ := New(Config{Strategy: StrategyRoundRobin})
		p.Register(m1)
		p.Register(m2)

		_, err := p.SendSMS(context.Background(), "+7999", "Test")
		if !errors.Is(err, ErrAllModemsFailed) {
			t.Errorf("expected ErrAllModemsFailed, got: %v", err)
		}
	})

	t.Run("invalid strategy returns ErrInvalidStrategy", func(t *testing.T) {
		_, err := New(Config{Strategy: "unknown_strategy"})
		if !errors.Is(err, ErrInvalidStrategy) {
			t.Errorf("expected ErrInvalidStrategy, got: %v", err)
		}
	})
}

func TestPool_Status(t *testing.T) {
	m1 := &mockMember{id: "m1", status: "ready", signal: 20, operator: "MTS"}
	m2 := &mockMember{id: "m2", status: "error", signal: 0, operator: "None"}

	p, _ := New(Config{Strategy: StrategyRoundRobin})
	p.Register(m1)
	p.Register(m2)

	st := p.Status()
	if st.TotalModems != 2 {
		t.Errorf("expected 2 total modems, got %d", st.TotalModems)
	}
	if st.ReadyModems != 1 {
		t.Errorf("expected 1 ready modem, got %d", st.ReadyModems)
	}
	if st.Strategy != StrategyRoundRobin {
		t.Errorf("expected StrategyRoundRobin, got %s", st.Strategy)
	}
}
