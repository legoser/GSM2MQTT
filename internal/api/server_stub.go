//go:build no_api

package api

import (
	"context"
)

// Server is a stub for when the API is disabled via build tags.
type Server struct{}

// NewServer returns a dummy server stub.
func NewServer(cfg ServerConfig, manager ModemManager) *Server {
	return &Server{}
}

// Start does nothing when the API is disabled.
func (s *Server) Start(ctx context.Context) error { return nil }

// Stop does nothing when the API is disabled.
func (s *Server) Stop(ctx context.Context) {}
