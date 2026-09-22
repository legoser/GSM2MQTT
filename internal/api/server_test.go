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
	dialNum  string
	hangup   bool
	inbox    []ReceivedSMS
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

func (m *mockModemManager) DialCall(ctx context.Context, modemID, number string) error {
	m.dialNum = number
	return nil
}

func (m *mockModemManager) HangupCall(ctx context.Context, modemID string) error {
	m.hangup = true
	return nil
}

func (m *mockModemManager) GetReceivedSMS() []ReceivedSMS {
	return m.inbox
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

func TestServer_Metrics(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 on /metrics, got %d", w.Code)
	}
	contentType := w.Header().Get("Content-Type")
	if !bytes.Contains([]byte(contentType), []byte("text/plain")) {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}
}

func TestServer_CallDial(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
		"number":   "+79001234567",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/call/dial", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.dialNum != "+79001234567" {
		t.Errorf("expected dialed number +79001234567, got %s", mock.dialNum)
	}
}

func TestServer_CallHangup(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/call/hangup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !mock.hangup {
		t.Errorf("expected hangup to be true")
	}
}

func TestServer_GetInbox(t *testing.T) {
	mock := &mockModemManager{
		inbox: []ReceivedSMS{
			{
				ID:        "msg-1",
				ModemID:   "modem1",
				Sender:    "+79998887766",
				Timestamp: "2026-09-22 12:00:00",
				Text:      "Test inbox message",
			},
		},
	}
	server := NewServer(ServerConfig{Port: 8080}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/sms/inbox", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res []ReceivedSMS
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if len(res) != 1 || res[0].Text != "Test inbox message" {
		t.Errorf("unexpected inbox response: %+v", res)
	}
}

