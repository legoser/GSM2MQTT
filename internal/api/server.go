package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/legoser/gsm2mqtt/internal/services"
)

// ServerConfig configures the embedded HTTP Web and REST server.
type ServerConfig struct {
	Host string
	Port int
}

// ModemSummary is an alias to services.ModemSummary for API presentation.
type ModemSummary = services.ModemSummary

// ModemManager is the interface required by the API to query state and dispatch operations.
type ModemManager interface {
	GetModems() []ModemSummary
	SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error)
	SendUSSD(ctx context.Context, modemID, code string) (string, error)
}

// Server provides Web UI and REST API endpoints for GSM2MQTT.
type Server struct {
	cfg     ServerConfig
	manager ModemManager
	mux     *http.ServeMux
}

// NewServer constructs a new API Server.
func NewServer(cfg ServerConfig, manager ModemManager) *Server {
	s := &Server{
		cfg:     cfg,
		manager: manager,
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the HTTP request handler for testing or routing.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start launches the HTTP server listening on the configured address until context cancellation.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/modems", s.handleGetModems)
	s.mux.HandleFunc("POST /api/sms/send", s.handleSendSMS)
	s.mux.HandleFunc("POST /api/ussd/send", s.handleSendUSSD)
	s.mux.HandleFunc("GET /", s.handleRootUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleGetModems(w http.ResponseWriter, r *http.Request) {
	modems := s.manager.GetModems()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(modems)
}

func (s *Server) handleSendSMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
		To      string `json:"to"`
		Text    string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	refs, err := s.manager.SendSMS(r.Context(), req.ModemID, req.To, req.Text)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"refs":    refs,
	})
}

func (s *Server) handleSendUSSD(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
		Code    string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	reply, err := s.manager.SendUSSD(r.Context(), req.ModemID, req.Code)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": reply,
	})
}

func (s *Server) handleRootUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html>
<head>
<title>GSM2MQTT Gateway Dashboard</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 2rem; background: #f8f9fa; color: #212529; }
.card { background: #fff; padding: 1.5rem; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 1.5rem; max-width: 600px; }
h1, h2 { color: #1a73e8; }
input, textarea, button { width: 100%; box-sizing: border-box; padding: 0.5rem; margin-top: 0.5rem; margin-bottom: 1rem; border: 1px solid #ced4da; border-radius: 4px; }
button { background: #1a73e8; color: #fff; border: none; font-weight: bold; cursor: pointer; }
button:hover { background: #1557b0; }
</style>
</head>
<body>
<h1>GSM2MQTT Gateway</h1>
<div class="card">
<h2>Modem Telemetry</h2>
<p>Gateway status: <strong>Online</strong></p>
<p><a href="/api/modems">View JSON Modems Telemetry</a></p>
</div>
<div class="card">
<h2>Send Test SMS</h2>
<form action="/api/sms/send" method="POST" onsubmit="event.preventDefault(); fetch('/api/sms/send', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({modem_id:document.getElementById('m').value, to:document.getElementById('to').value, text:document.getElementById('txt').value})}).then(r=>r.json()).then(d=>alert(JSON.stringify(d)));">
<label>Modem ID: <input id="m" value="modem1" required></label>
<label>Phone Number: <input id="to" placeholder="+79001234567" required></label>
<label>Message Text: <textarea id="txt" rows="3" required></textarea></label>
<button type="submit">Send SMS</button>
</form>
</div>
</body>
</html>`
	_, _ = w.Write([]byte(html))
}
