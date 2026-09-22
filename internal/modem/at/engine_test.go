package at

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
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
	closed bool
}

func newMockPort() *mockPort {
	m := &mockPort{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *mockPort) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.EOF
	}
	return m.inBuf.Write(p)
}

func (m *mockPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for m.outBuf.Len() == 0 && !m.closed {
		m.cond.Wait()
	}
	if m.closed && m.outBuf.Len() == 0 {
		return 0, io.EOF
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

func (m *mockPort) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.cond.Broadcast()
	return nil
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

func TestEngine_Send_IntermediateLines(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\n+CSQ: 21,99\r\n\r\nOK\r\n")
	}()

	resp, err := engine.Send("AT+CSQ", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK = true")
	}
	if len(resp.Lines) != 1 || resp.Lines[0] != "+CSQ: 21,99" {
		t.Errorf("expected lines ['+CSQ: 21,99'], got %v", resp.Lines)
	}
}

func TestEngine_Send_CME_Error(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\n+CME ERROR: 10\r\n")
	}()

	resp, err := engine.Send("AT+CPIN?", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Error {
		t.Errorf("expected Error = true")
	}
	if len(resp.Lines) == 0 || !strings.Contains(resp.Lines[0], "+CME ERROR: 10") {
		t.Errorf("expected CME error line, got %v", resp.Lines)
	}
}

func TestEngine_Send_Timeout(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// No response fed, send should timeout
	_, err := engine.Send("AT", 50*time.Millisecond)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got: %v", err)
	}
}

func TestEngine_SendCommand(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nSIEMENS\r\nTC35\r\nOK\r\n")
	}()

	out, err := engine.SendCommand(ctx, "ATI")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "SIEMENS") || !strings.Contains(out, "TC35") {
		t.Errorf("expected output to contain SIEMENS and TC35, got: %q", out)
	}
}

var _ io.ReadWriter = (*mockPort)(nil)
