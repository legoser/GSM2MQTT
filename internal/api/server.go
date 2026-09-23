package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

// CallStatus is an alias to services.CallStatus for API presentation.
type CallStatus = services.CallStatus

// MQTTStatus is an alias to services.MQTTStatus for API presentation.
type MQTTStatus = services.MQTTStatus

// ModemManager is the interface required by the API to query state and dispatch operations.
type ModemManager interface {
	GetModems() []ModemSummary
	SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error)
	SendUSSD(ctx context.Context, modemID, code string) (string, error)
	DialCall(ctx context.Context, modemID, number string) error
	HangupCall(ctx context.Context, modemID string) error
	GetCallStatus(modemID string) CallStatus
	SendRawAT(ctx context.Context, modemID, cmd string) (string, error)
	GetReceivedSMS() []ReceivedSMS
	GetMQTTStatus() MQTTStatus
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
	s.mux.HandleFunc("GET /api/call/status", s.handleCallStatus)
	s.mux.HandleFunc("GET /api/mqtt/status", s.handleMQTTStatus)
	s.mux.HandleFunc("POST /api/at/send", s.handleSendAT)
	s.mux.HandleFunc("GET /api/sms/inbox", s.handleGetInbox)
	s.mux.HandleFunc("GET /favicon.ico", s.handleFavicon)
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
		slog.Warn("api send sms invalid json", slog.Any("error", err))
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	slog.Info("api send sms requested", slog.String("modem", req.ModemID), slog.String("to", req.To), slog.Int("len", len(req.Text)))
	refs, err := s.manager.SendSMS(r.Context(), req.ModemID, req.To, req.Text)
	if err != nil {
		slog.Error("api send sms failed", slog.String("modem", req.ModemID), slog.String("to", req.To), slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("api send sms succeeded", slog.String("modem", req.ModemID), slog.String("to", req.To), slog.Any("refs", refs))
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
		slog.Warn("api send ussd invalid json", slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, "invalid request json")
		return
	}

	slog.Info("api send ussd requested", slog.String("modem", req.ModemID), slog.String("code", req.Code))
	reply, err := s.manager.SendUSSD(r.Context(), req.ModemID, req.Code)
	if err != nil {
		slog.Error("api send ussd failed", slog.String("modem", req.ModemID), slog.String("code", req.Code), slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("api send ussd succeeded", slog.String("modem", req.ModemID), slog.String("code", req.Code), slog.String("reply", reply))
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
		slog.Warn("api call dial invalid json", slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, "invalid request json")
		return
	}

	slog.Info("api call dial requested", slog.String("modem", req.ModemID), slog.String("number", req.Number))
	if err := s.manager.DialCall(r.Context(), req.ModemID, req.Number); err != nil {
		slog.Error("api call dial failed", slog.String("modem", req.ModemID), slog.String("number", req.Number), slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("api call dial succeeded", slog.String("modem", req.ModemID), slog.String("number", req.Number))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleCallHangup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("api call hangup invalid json", slog.Any("error", err))
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}

	slog.Info("api call hangup requested", slog.String("modem", req.ModemID))
	if err := s.manager.HangupCall(r.Context(), req.ModemID); err != nil {
		slog.Error("api call hangup failed", slog.String("modem", req.ModemID), slog.Any("error", err))
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	slog.Info("api call hangup succeeded", slog.String("modem", req.ModemID))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleCallStatus(w http.ResponseWriter, r *http.Request) {
	modemID := r.URL.Query().Get("modem_id")
	status := s.manager.GetCallStatus(modemID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleMQTTStatus(w http.ResponseWriter, r *http.Request) {
	status := s.manager.GetMQTTStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleSendAT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
		return
	}
	reply, err := s.manager.SendRawAT(r.Context(), req.ModemID, req.Command)
	if err != nil {
		slog.Error("api send at failed", slog.String("modem", req.ModemID), slog.String("command", req.Command), slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	slog.Info("api send at succeeded", slog.String("modem", req.ModemID), slog.String("command", req.Command), slog.String("reply", reply))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"reply":   reply,
	})
}

func (s *Server) handleGetInbox(w http.ResponseWriter, r *http.Request) {
	msgs := s.manager.GetReceivedSMS()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write(getFaviconSVG())
}

func (s *Server) handleRootUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(getDashboardHTML())
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
