// Package at implements the AT command communication engine.
package at

import (
	"context"
	"io"
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
	port    io.ReadWriter
	urcChan chan string
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
	// STUB for TDD: will fail tests
	return nil, nil
}

// Start starts the background read loop for handling responses and URCs.
func (e *Engine) Start(ctx context.Context) error {
	// STUB for TDD: will fail tests
	return nil
}
