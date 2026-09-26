//go:build no_api

package api

import (
	"context"
	"testing"
)

func TestServerStub(t *testing.T) {
	srv := NewServer(ServerConfig{}, nil)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("unexpected error from stub Start: %v", err)
	}
	srv.Stop(context.Background())
}
