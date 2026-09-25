package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServer_Auth_Table covers the Bearer token gate: fail-open when no
// token is configured, 401 on missing/wrong token, 200 on valid token.
func TestServer_Auth_Table(t *testing.T) {
	newReq := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/api/modems", nil)
	}

	t.Run("empty token rejects without header", func(t *testing.T) {
		server := NewServer(ServerConfig{Port: 8080}, &mockModemManager{})
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, newReq())
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 with empty token, got %d", w.Code)
		}
	})

	t.Run("wrong token rejected", func(t *testing.T) {
		server := NewServer(ServerConfig{Port: 8080, Token: "secret123"}, &mockModemManager{})
		req := newReq()
		req.Header.Set("Authorization", "Bearer wrong")
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("missing header rejected", func(t *testing.T) {
		server := NewServer(ServerConfig{Port: 8080, Token: "secret123"}, &mockModemManager{})
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, newReq())
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("malformed scheme rejected", func(t *testing.T) {
		server := NewServer(ServerConfig{Port: 8080, Token: "secret123"}, &mockModemManager{})
		req := newReq()
		req.Header.Set("Authorization", "Token secret123")
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid token allowed", func(t *testing.T) {
		server := NewServer(ServerConfig{Port: 8080, Token: "secret123"}, &mockModemManager{})
		req := newReq()
		req.Header.Set("Authorization", "Bearer secret123")
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("token prefix trick rejected", func(t *testing.T) {
		server := NewServer(ServerConfig{Port: 8080, Token: "secret"}, &mockModemManager{})
		req := newReq()
		req.Header.Set("Authorization", "Bearer secret123")
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for prefix trick, got %d", w.Code)
		}
	})
}

// TestServer_Auth_POSTProtected verifies a mutating endpoint also requires auth.
func TestServer_Auth_POSTProtected(t *testing.T) {
	body := `{"modem_id":"m1","command":"ATI"}`
	newReq := func() *http.Request {
		return httptest.NewRequest(http.MethodPost, "/api/at/send", strings.NewReader(body))
	}

	server := NewServer(ServerConfig{Port: 8080, Token: "secret123"}, &mockModemManager{})

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, newReq())
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w.Code)
	}

	req := newReq()
	req.Header.Set("Authorization", "Bearer secret123")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestServer_BodyLimit verifies oversized JSON bodies are rejected (DoS guard).
func TestServer_BodyLimit(t *testing.T) {
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, &mockModemManager{})
	big := strings.Repeat("A", (64<<10)+1024)
	req := httptest.NewRequest(http.MethodPost, "/api/sms/send",
		strings.NewReader(`{"modem_id":"m1","to":"123","text":"`+big+`"}`))
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("expected oversized body to be rejected, got 200")
	}
}

// TestServer_SecurityHeaders verifies baseline hardening headers.
func TestServer_SecurityHeaders(t *testing.T) {
	server := NewServer(ServerConfig{Port: 8080}, &mockModemManager{})
	for _, path := range []string{"/", "/health", "/api/modems"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, req)
		if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", path, got)
		}
		if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", path, got)
		}
		if got := w.Header().Get("Content-Security-Policy"); got == "" {
			t.Errorf("%s: missing Content-Security-Policy", path)
		}
	}
}
