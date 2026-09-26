//go:build !no_api

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/services"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

type mockModemManager struct {
	modems       []ModemSummary
	sentSMS      map[string]string
	sentUSSD     string
	dialNum      string
	hangup       bool
	inbox        []ReceivedSMS
	tariffStatus *tariff.UsageStatus
	tariffConfig tariff.Config
	tariffReset  bool
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

func (m *mockModemManager) GetCallStatus(modemID string) CallStatus {
	return CallStatus{
		State:   services.CallStateRinging,
		Message: "Ringing",
		Logs: []services.CallLogEntry{
			{Time: time.Now(), Message: "Dialing..."},
			{Time: time.Now(), Message: "Ringing..."},
		},
	}
}

func (m *mockModemManager) SendRawAT(ctx context.Context, modemID, cmd string) (string, error) {
	return "OK", nil
}

func (m *mockModemManager) GetReceivedSMS() []ReceivedSMS {
	return m.inbox
}

func (m *mockModemManager) GetMQTTStatus() MQTTStatus {
	return MQTTStatus{
		Connected:   true,
		Broker:      "tcp://mosquitto:1883",
		Port:        1883,
		ClientID:    "gsm2mqtt-test",
		TopicPrefix: "gsm2mqtt",
	}
}

func (m *mockModemManager) UpdateTariffConfig(modemID string, cfg tariff.Config) error {
	m.tariffConfig = cfg
	return nil
}

func (m *mockModemManager) SetTariffUsage(modemID string, update tariff.UsageUpdate) error {
	return nil
}

func (m *mockModemManager) ResetTariffQuotas(modemID string) error {
	m.tariffReset = true
	return nil
}

func (m *mockModemManager) GetTariffStatus(modemID string) (*tariff.UsageStatus, error) {
	if m.tariffStatus != nil {
		return m.tariffStatus, nil
	}
	return &tariff.UsageStatus{
		Balance:       123.45,
		Currency:      "RUB",
		SMSMonthCount: 15,
		SMSLimit:      100,
		SMSRemaining:  85,
	}, nil
}

func TestServer_Health(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/modems", nil)
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
		"to":       "+79991112233",
		"text":     "Web test message",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/sms/send", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
		"code":     "*100#",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ussd/send", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !bytes.Contains([]byte(ct), []byte("text/html")) {
		t.Errorf("expected text/html content type, got: %s", ct)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("GSM2MQTT Control Center")) {
		t.Errorf("expected HTML to contain GSM2MQTT Control Center, got: %s", w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("📟")) {
		t.Errorf("expected HTML to contain pager emoji, got: %s", w.Body.String())
	}
}

func TestServer_Metrics(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
		"number":   "+79001234567",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/call/dial", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "modem1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/call/hangup", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
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
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/sms/inbox", nil)
	req.Header.Set("Authorization", "Bearer test")
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

func TestServer_SendAT(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Port: 8080, Token: "test"}, mock)

	body, _ := json.Marshal(map[string]string{
		"modem_id": "test_modem",
		"command":  "AT+CSQ",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/at/send", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if res["reply"] != "OK" {
		t.Errorf("expected OK reply, got: %v", res)
	}
}

func TestServer_GetCallStatus(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Host: "127.0.0.1", Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/call/status?modem_id=test_modem", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res CallStatus
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if res.State != services.CallStateRinging {
		t.Errorf("expected ringing state, got: %v", res.State)
	}
	if len(res.Logs) != 2 {
		t.Errorf("expected 2 log entries, got: %d", len(res.Logs))
	}
}

func TestServer_Favicon(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Host: "127.0.0.1", Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !bytes.Contains([]byte(ct), []byte("image/svg+xml")) {
		t.Errorf("expected image/svg+xml content type, got: %s", ct)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("📟")) {
		t.Errorf("expected favicon SVG to contain pager emoji")
	}
}

func TestServer_GetMQTTStatus(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Host: "127.0.0.1", Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/mqtt/status", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res MQTTStatus
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !res.Connected {
		t.Error("expected connected true")
	}
	if res.Broker != "tcp://mosquitto:1883" {
		t.Errorf("expected broker tcp://mosquitto:1883, got %s", res.Broker)
	}
}

func TestServer_TariffStatus(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Host: "127.0.0.1", Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/tariff/status?modem_id=modem1", nil)
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res tariff.UsageStatus
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal tariff status: %v", err)
	}
	if res.SMSRemaining != 85 || res.SMSLimit != 100 {
		t.Errorf("unexpected tariff status response: %+v", res)
	}
}

func TestServer_TariffConfig(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Host: "127.0.0.1", Port: 8080, Token: "test"}, mock)

	smsLimit := 250
	reqBody, _ := json.Marshal(map[string]interface{}{
		"modem_id":  "modem1",
		"sms_limit": smsLimit,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/tariff/config", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.tariffConfig.SMSLimit != 250 {
		t.Errorf("expected SMSLimit 250, got %d", mock.tariffConfig.SMSLimit)
	}
}

func TestServer_TariffReset(t *testing.T) {
	mock := &mockModemManager{}
	server := NewServer(ServerConfig{Host: "127.0.0.1", Port: 8080, Token: "test"}, mock)

	req := httptest.NewRequest(http.MethodPost, "/api/tariff/reset", bytes.NewReader([]byte(`{"modem_id":"modem1"}`)))
	req.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !mock.tariffReset {
		t.Errorf("expected tariffReset to be true")
	}
}
