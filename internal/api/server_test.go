package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/legoser/gsm2mqtt/internal/services"
)

type mockModemManager struct {
	modems   []ModemSummary
	sentSMS  map[string]string
	sentUSSD string
}

func (m *mockModemManager) GetModems() []ModemSummary {
	return m.modems
}

func (m *mockModemManager) SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error) {
	if m.sentSMS == nil {
		m.sentSMS = make(map[string]string)
	}
	m.sentSMS[to] = text
	return []byte{42}, nil
}

func (m *mockModemManager) SendUSSD(ctx context.Context, modemID, code string) (string, error) {
	m.sentUSSD = code
	return "Balance is 100 RUB", nil
}

func TestServer_Health(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Errorf("expected body to contain ok status, got: %s", w.Body.String())
	}
}

func TestServer_GetModems(t *testing.T) {
	mock := &mockModemManager{
		modems: []ModemSummary{
			{
				ID:   "modem1",
				Type: "siemens",
				Health: services.ModemHealth{
					Status:    "ready",
					Signal:    20,
					SignalDBm: -73,
					Operator:  "MTS",
					SIM:       "READY",
				},
				Balance:  150.0,
				Currency: "RUB",
			},
		},
	}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/modems", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp []ModemSummary
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if len(resp) != 1 || resp[0].ID != "modem1" {
		t.Errorf("unexpected modems response: %+v", resp)
	}
}

func TestServer_SendSMS(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
		"to":       "+79991112233",
		"text":     "Web test message",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/sms/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.sentSMS["+79991112233"] != "Web test message" {
		t.Errorf("expected SMS to be sent, got: %v", mock.sentSMS)
	}
}

func TestServer_SendUSSD(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
		"code":     "*100#",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ussd/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.sentUSSD != "*100#" {
		t.Errorf("expected USSD code *100#, got: %q", mock.sentUSSD)
	}
}

func TestServer_RootUI(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("GSM2MQTT")) {
		t.Errorf("expected HTML title mentioning GSM2MQTT, got: %s", w.Body.String())
	}
}
