// Package at implements the AT command communication engine.
package at

import (
	"context"
	"errors"
	"io"
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

	fullCmd := cleanCmd + "\r\n"
	if _, err := e.port.Write([]byte(fullCmd)); err != nil {
		e.clearInFlight()
		return nil, err
	}

	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(timeout):
		e.clearInFlight()
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
			if b == '\n' {
				line := strings.TrimSpace(string(lineBuf))
				lineBuf = lineBuf[:0]
				if line != "" {
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
	e.respMu.Unlock()
}

func (e *Engine) processLine(line string) {
	if isURC(line) {
		e.dispatchURC(line)
		return
	}

	e.respMu.Lock()
	defer e.respMu.Unlock()

	if e.inFlight == nil {
		e.dispatchURC(line)
		return
	}

	if line == e.activeCmd {
		return // Ignore command echo
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

func (e *Engine) dispatchURC(line string) {
	select {
	case e.urcChan <- line:
	default:
	}
}

func isURC(line string) bool {
	urcPrefixes := []string{
		"+CLIP:", "+CMTI:", "+CMT:", "+CDS:", "+DTMF:",
		"+CUSD:", "RING", "+CRING:", "+CREG:",
		"+CGREG:", "+CEREG:", "NO CARRIER",
	}
	for _, p := range urcPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}
