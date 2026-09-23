// Package at implements the AT command communication engine.
package at

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Response represents a modem response to an AT command.
type Response struct {
	Lines []string
	OK    bool
	Error bool
}

// Engine manages AT command execution and URC processing over an io.ReadWriter.
type Engine struct {
	port        io.ReadWriter
	urcChan     chan string
	cmdMu       sync.Mutex
	respMu      sync.Mutex
	inFlight    chan *Response
	currentResp *Response
	activeCmd   string
	promptChan  chan struct{}
}

// NewEngine creates a new AT command engine.
func NewEngine(port io.ReadWriter) *Engine {
	return &Engine{
		port:    port,
		urcChan: make(chan string, 100),
	}
}

// URC returns the receive-only channel for Unsolicited Result Codes.
func (e *Engine) URC() <-chan string {
	return e.urcChan
}

// Send sends an AT command and waits for a final response (OK / ERROR) or timeout.
func (e *Engine) Send(cmd string, timeout time.Duration) (*Response, error) {
	e.cmdMu.Lock()
	defer e.cmdMu.Unlock()

	respChan := make(chan *Response, 1)
	cleanCmd := strings.TrimSpace(cmd)

	e.respMu.Lock()
	e.inFlight = respChan
	e.currentResp = &Response{}
	e.activeCmd = cleanCmd
	e.respMu.Unlock()

	slog.Debug("AT command TX", slog.String("cmd", sanitizeAT(cleanCmd)))

	fullCmd := cleanCmd + "\r\n"
	if _, err := e.port.Write([]byte(fullCmd)); err != nil {
		e.clearInFlight()
		return nil, err
	}

	select {
	case resp := <-respChan:
		slog.Debug("AT command RX",
			slog.String("cmd", sanitizeAT(cleanCmd)),
			slog.Bool("ok", resp.OK),
			slog.Bool("error", resp.Error),
			slog.Any("lines", resp.Lines),
		)
		return resp, nil
	case <-time.After(timeout):
		e.clearInFlight()
		slog.Warn("AT command timeout",
			slog.String("cmd", sanitizeAT(cleanCmd)),
			slog.Duration("timeout", timeout),
		)
		return nil, ErrTimeout
	}
}

// SendCommand executes an AT command with context cancellation, returning the text response.
func (e *Engine) SendCommand(ctx context.Context, cmd string) (string, error) {
	timeout := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", context.DeadlineExceeded
		}
		timeout = remaining
	}

	resp, err := e.Send(cmd, timeout)
	if err != nil {
		return "", err
	}
	output := strings.Join(resp.Lines, "\n")
	if resp.Error {
		return output, ErrCommandFailed
	}
	return output, nil
}

// SendPDU transmits an SMS PDU via AT+CMGS, waits for the '>' prompt, then sends the PDU payload with Ctrl+Z (\x1A).
func (e *Engine) SendPDU(cmdLength int, pduHex string, timeout time.Duration) (*Response, error) {
	e.cmdMu.Lock()
	defer e.cmdMu.Unlock()

	respChan := make(chan *Response, 1)
	promptChan := make(chan struct{}, 1)

	e.respMu.Lock()
	e.inFlight = respChan
	e.promptChan = promptChan
	e.currentResp = &Response{}
	e.activeCmd = "AT+CMGS"
	e.respMu.Unlock()

	defer e.clearInFlight()

	slog.Debug("AT sending PDU header", slog.Int("cmd_length", cmdLength))

	// 1. Send AT+CMGS=<length>\r
	cmd := fmt.Sprintf("AT+CMGS=%d\r", cmdLength)
	if _, err := e.port.Write([]byte(cmd)); err != nil {
		return nil, err
	}

	// 2. Wait for '>' prompt or early error
	select {
	case <-promptChan:
		slog.Debug("AT prompt received ('>')")
	case resp := <-respChan:
		return resp, nil
	case <-time.After(3 * time.Second):
	}

	slog.Debug("AT PDU payload transmitted", slog.Int("pdu_hex_len", len(pduHex)))

	// 3. Write PDU and Ctrl-Z (\x1A)
	if _, err := e.port.Write([]byte(pduHex + "\x1A")); err != nil {
		_, _ = e.port.Write([]byte("\x1B"))
		return nil, err
	}

	// 4. Wait for final response (+CMGS: <ref> and OK)
	select {
	case resp := <-respChan:
		slog.Debug("AT PDU final response", slog.Bool("ok", resp.OK), slog.Bool("error", resp.Error), slog.Any("lines", resp.Lines))
		return resp, nil
	case <-time.After(timeout):
		_, _ = e.port.Write([]byte("\x1B"))
		slog.Warn("AT PDU transmission timeout", slog.Duration("timeout", timeout))
		return nil, ErrTimeout
	}
}

func sanitizeAT(cmd string) string {
	if strings.Contains(strings.ToUpper(cmd), "CPIN") {
		return "AT+CPIN=***"
	}
	return cmd
}

// Start starts the background read loop for handling responses and URCs.
func (e *Engine) Start(ctx context.Context) error {
	buf := make([]byte, 256)
	var lineBuf []byte

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := e.port.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		for i := 0; i < n; i++ {
			b := buf[i]
			if b == '>' {
				e.respMu.Lock()
				ch := e.promptChan
				e.respMu.Unlock()
				if ch != nil {
					select {
					case ch <- struct{}{}:
					default:
					}
				}
				lineBuf = lineBuf[:0]
				continue
			}
			if b == '\n' {
				line := strings.TrimSpace(string(lineBuf))
				lineBuf = lineBuf[:0]
				if line != "" && line != ">" {
					e.processLine(line)
				}
			} else if b != '\r' {
				lineBuf = append(lineBuf, b)
			}
		}
	}
}

func (e *Engine) clearInFlight() {
	e.respMu.Lock()
	e.inFlight = nil
	e.currentResp = nil
	e.activeCmd = ""
	e.promptChan = nil
	e.respMu.Unlock()
}

func (e *Engine) processLine(line string) {
	e.respMu.Lock()
	defer e.respMu.Unlock()

	if e.inFlight == nil {
		e.dispatchURC(line)
		return
	}

	if line == e.activeCmd {
		return // Ignore command echo
	}

	if isCallTerminationResponse(e.activeCmd, line) {
		e.currentResp.Error = true
		e.currentResp.Lines = append(e.currentResp.Lines, line)
		e.inFlight <- e.currentResp
		e.inFlight = nil
		return
	}

	if !isCommandResponse(e.activeCmd, line) && isURC(line) {
		e.dispatchURC(line)
		return
	}

	switch {
	case line == "OK":
		e.currentResp.OK = true
		e.inFlight <- e.currentResp
		e.inFlight = nil
	case line == "ERROR", strings.HasPrefix(line, "+CME ERROR:"), strings.HasPrefix(line, "+CMS ERROR:"):
		e.currentResp.Error = true
		e.currentResp.Lines = append(e.currentResp.Lines, line)
		e.inFlight <- e.currentResp
		e.inFlight = nil
	default:
		e.currentResp.Lines = append(e.currentResp.Lines, line)
	}
}
