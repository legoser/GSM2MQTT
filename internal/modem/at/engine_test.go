package at

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

// mockPort implements io.ReadWriter for testing AT command exchange.
type mockPort struct {
	mu     sync.Mutex
	inBuf  bytes.Buffer // Commands written by engine to modem
	outBuf bytes.Buffer // Responses read by engine from modem
	cond   *sync.Cond
}

func newMockPort() *mockPort {
	m := &mockPort{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *mockPort) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inBuf.Write(p)
}

func (m *mockPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for m.outBuf.Len() == 0 {
		m.cond.Wait()
	}
	return m.outBuf.Read(p)
}

// FeedResponse feeds data to be read by the engine.
func (m *mockPort) FeedResponse(data string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outBuf.WriteString(data)
	m.cond.Broadcast()
}

func TestEngine_Send_OK(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// Simulate modem responding with OK
	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nOK\r\n")
	}()

	resp, err := engine.Send("AT", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !resp.OK {
		t.Errorf("expected OK to be true")
	}
}

func TestEngine_Send_Error(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nERROR\r\n")
	}()

	resp, err := engine.Send("AT+UNKNOWN", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !resp.Error {
		t.Errorf("expected Error to be true")
	}
}

func TestEngine_URC(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// Feed unsolicited call notification
	go func() {
		time.Sleep(20 * time.Millisecond)
		port.FeedResponse("\r\n+CLIP: \"+79991112233\",145\r\n")
	}()

	select {
	case urc := <-engine.URC():
		expected := "+CLIP: \"+79991112233\",145"
		if urc != expected {
			t.Errorf("expected URC %q, got %q", expected, urc)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for URC")
	}
}

var _ io.ReadWriter = (*mockPort)(nil)
