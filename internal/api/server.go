package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/legoser/gsm2mqtt/internal/metrics"
	"github.com/legoser/gsm2mqtt/internal/services"
)

// ServerConfig configures the embedded HTTP Web and REST server.
type ServerConfig struct {
	Host string
	Port int
}

// ModemSummary is an alias to services.ModemSummary for API presentation.
type ModemSummary = services.ModemSummary

// ReceivedSMS is an alias to services.ReceivedSMS for API presentation.
type ReceivedSMS = services.ReceivedSMS

// ModemManager is the interface required by the API to query state and dispatch operations.
type ModemManager interface {
	GetModems() []ModemSummary
	SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error)
	SendUSSD(ctx context.Context, modemID, code string) (string, error)
	DialCall(ctx context.Context, modemID, number string) error
	HangupCall(ctx context.Context, modemID string) error
	GetReceivedSMS() []ReceivedSMS
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
	s.mux.Handle("GET /metrics", metrics.DefaultRegistry.Handler())
	s.mux.HandleFunc("GET /api/modems", s.handleGetModems)
	s.mux.HandleFunc("POST /api/sms/send", s.handleSendSMS)
	s.mux.HandleFunc("POST /api/ussd/send", s.handleSendUSSD)
	s.mux.HandleFunc("POST /api/call/dial", s.handleCallDial)
	s.mux.HandleFunc("POST /api/call/hangup", s.handleCallHangup)
	s.mux.HandleFunc("GET /api/sms/inbox", s.handleGetInbox)
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

func (s *Server) handleCallDial(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
		Number  string `json:"number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}
	if err := s.manager.DialCall(r.Context(), req.ModemID, req.Number); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleCallHangup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.manager.HangupCall(r.Context(), req.ModemID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleGetInbox(w http.ResponseWriter, r *http.Request) {
	msgs := s.manager.GetReceivedSMS()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

func (s *Server) handleRootUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}
