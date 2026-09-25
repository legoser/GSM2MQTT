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

func TestEngine_Send_CREG_PreservedAsResponse(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\n+CREG: 0,1\r\n\r\nOK\r\n")
	}()

	resp, err := engine.Send("AT+CREG?", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK = true")
	}
	if len(resp.Lines) != 1 || resp.Lines[0] != "+CREG: 0,1" {
		t.Fatalf("expected lines to contain '+CREG: 0,1', got: %v", resp.Lines)
	}
}

func TestEngine_Dial_CallFailureResponses(t *testing.T) {
	testCases := []struct {
		name     string
		response string
	}{
		{"NoCarrier", "NO CARRIER"},
		{"Busy", "BUSY"},
		{"NoAnswer", "NO ANSWER"},
		{"NoDialtone", "NO DIALTONE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port := newMockPort()
			engine := NewEngine(port)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				_ = engine.Start(ctx)
			}()

			go func() {
				time.Sleep(10 * time.Millisecond)
				port.FeedResponse("\r\n" + tc.response + "\r\n")
			}()

			resp, err := engine.Send("ATD+79991112233;", 200*time.Millisecond)
			if err != nil {
				t.Fatalf("unexpected error (likely timeout waiting for response): %v", err)
			}
			if !resp.Error {
				t.Fatalf("expected resp.Error to be true, got false")
			}
			if len(resp.Lines) == 0 || resp.Lines[0] != tc.response {
				t.Fatalf("expected lines to contain %q, got: %v", tc.response, resp.Lines)
			}
		})
	}
}

func TestEngine_NoCarrier_AsURCWhenIdle(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nNO CARRIER\r\n")
	}()

	select {
	case urc := <-engine.URC():
		if urc != "NO CARRIER" {
			t.Errorf("expected NO CARRIER URC, got: %q", urc)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for NO CARRIER URC")
	}
}

func TestEngine_HuaweiURC_DuringInFlight(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\n^ORIG: 1, 0\r\n")
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nOK\r\n")
	}()

	resp, err := engine.Send("ATD+79991112233;", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected resp.OK to be true")
	}

	select {
	case urc := <-engine.URC():
		if urc != "^ORIG: 1, 0" {
			t.Errorf("expected ^ORIG: 1, 0, got %q", urc)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for Huawei URC")
	}
}

func TestEngine_SendPDU_Success(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		// Wait until port has received "AT+CMGS=15\r"
		for {
			port.mu.Lock()
			hasCmd := strings.Contains(port.inBuf.String(), "AT+CMGS=15\r")
			port.mu.Unlock()
			if hasCmd {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Feed prompt
		port.FeedResponse("\r\n> ")

		// Wait until port has received PDU with \x1A
		for {
			port.mu.Lock()
			hasPDU := strings.Contains(port.inBuf.String(), "0011000B91\x1A")
			port.mu.Unlock()
			if hasPDU {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Feed success response
		port.FeedResponse("\r\n+CMGS: 42\r\n\r\nOK\r\n")
	}()

	resp, err := engine.SendPDU(context.Background(), 15, "0011000B91", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected SendPDU error: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected resp.OK to be true")
	}
	if len(resp.Lines) == 0 || resp.Lines[0] != "+CMGS: 42" {
		t.Fatalf("expected +CMGS: 42 in lines, got: %v", resp.Lines)
	}
}

func TestEngine_SendPDU_EarlyError(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\n+CMS ERROR: 304\r\n")
	}()

	resp, err := engine.SendPDU(context.Background(), 15, "0011000B91", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Error {
		t.Fatalf("expected resp.Error to be true")
	}
	if len(resp.Lines) == 0 || resp.Lines[0] != "+CMS ERROR: 304" {
		t.Fatalf("expected +CMS ERROR: 304, got: %v", resp.Lines)
	}
}

func TestEngine_SendPDU_Timeout(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// Feed prompt but never send final OK/CMGS
	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\n> ")
	}()

	_, err := engine.SendPDU(context.Background(), 15, "0011000B91", 50*time.Millisecond)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got: %v", err)
	}

	// Verify ESC was sent to port on timeout
	port.mu.Lock()
	inStr := port.inBuf.String()
	port.mu.Unlock()
	if !strings.Contains(inStr, "\x1B") {
		t.Fatalf("expected \\x1B (ESC) written to port on timeout, got: %q", inStr)
	}
}

func TestEngine_SendPDU_PromptTimeout(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// Never feed the '>' prompt: engine must abort with ESC and must NOT
	// blind-send the PDU payload (regression: old code fell through).
	const pduHex = "0011000B91AA"
	_, err := engine.SendPDU(context.Background(), 15, pduHex, 500*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("expected prompt timeout error, got: %v", err)
	}

	port.mu.Lock()
	inStr := port.inBuf.String()
	port.mu.Unlock()
	if !strings.Contains(inStr, "\x1B") {
		t.Fatalf("expected \\x1B (ESC) written to port on prompt timeout, got: %q", inStr)
	}
	if strings.Contains(inStr, pduHex) {
		t.Fatalf("PDU payload must not be sent without '>' prompt, port got: %q", inStr)
	}
}

func TestEngine_SIMComURC_DuringInFlight(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// Simulate SIM800 URC notifications arriving while AT+CSQ is executing
	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nCall Ready\r\n")
		port.FeedResponse("\r\nSMS Ready\r\n")
		port.FeedResponse("\r\n+CSQ: 24,0\r\nOK\r\n")
	}()

	resp, err := engine.Send("AT+CSQ", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK")
	}

	// Ensure Call Ready and SMS Ready were dispatched to URC and not included in CSQ response
	if len(resp.Lines) != 1 || resp.Lines[0] != "+CSQ: 24,0" {
		t.Errorf("expected lines to contain only [+CSQ: 24,0], got: %v", resp.Lines)
	}

	select {
	case urc := <-engine.URC():
		if urc != "Call Ready" {
			t.Errorf("expected first URC 'Call Ready', got: %q", urc)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for Call Ready URC")
	}

	select {
	case urc := <-engine.URC():
		if urc != "SMS Ready" {
			t.Errorf("expected second URC 'SMS Ready', got: %q", urc)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for SMS Ready URC")
	}
}

func TestEngine_NeowayURC_DuringInFlight(t *testing.T) {
	port := newMockPort()
	engine := NewEngine(port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engine.Start(ctx)
	}()

	// Simulate Neoway startup URC lines arriving while AT+CSQ is running
	go func() {
		time.Sleep(10 * time.Millisecond)
		port.FeedResponse("\r\nMODEM:STARTUP\r\n")
		port.FeedResponse("\r\n+PBREADY\r\n")
		port.FeedResponse("\r\n+CSQ: 18,0\r\nOK\r\n")
	}()

	resp, err := engine.Send("AT+CSQ", 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK")
	}

	if len(resp.Lines) != 1 || resp.Lines[0] != "+CSQ: 18,0" {
		t.Errorf("expected lines to contain only [+CSQ: 18,0], got: %v", resp.Lines)
	}

	select {
	case urc := <-engine.URC():
		if urc != "MODEM:STARTUP" {
			t.Errorf("expected first URC 'MODEM:STARTUP', got: %q", urc)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for MODEM:STARTUP URC")
	}

	select {
	case urc := <-engine.URC():
		if urc != "+PBREADY" {
			t.Errorf("expected second URC '+PBREADY', got: %q", urc)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for +PBREADY URC")
	}
}

var _ io.ReadWriter = (*mockPort)(nil)
