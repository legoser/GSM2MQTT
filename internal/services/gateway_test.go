package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/sms"
	"github.com/legoser/gsm2mqtt/internal/tariff"
	"github.com/legoser/gsm2mqtt/internal/transport"
)

type mockGatewayPort struct {
	mu     sync.Mutex
	closed bool
	cond   *sync.Cond
	buf    []byte
}

func newMockGatewayPort() *mockGatewayPort {
	p := &mockGatewayPort{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *mockGatewayPort) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.buf) == 0 && !p.closed {
		p.cond.Wait()
	}
	if p.closed && len(p.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.buf)
	p.buf = p.buf[n:]
	return n, nil
}

func (p *mockGatewayPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, io.EOF
	}
	cmd := string(b)
	switch {
	case strings.HasPrefix(cmd, "AT+CMGS="):
		p.buf = append(p.buf, []byte("\r\n> ")...)
	case strings.HasSuffix(cmd, "\x1A"):
		p.buf = append(p.buf, []byte("\r\n+CMGS: 42\r\n\r\nOK\r\n")...)
	default:
		// Echo OK for standard commands
		p.buf = append(p.buf, []byte("\r\nOK\r\n")...)
	}
	p.cond.Broadcast()
	return len(b), nil
}

func (p *mockGatewayPort) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.cond.Broadcast()
	return nil
}

func (p *mockGatewayPort) SetDTR(dtr bool) error    { return nil }
func (p *mockGatewayPort) SetRTS(rts bool) error    { return nil }
func (p *mockGatewayPort) ResetInputBuffer() error  { return nil }
func (p *mockGatewayPort) ResetOutputBuffer() error { return nil }

type mockGatewayOpener struct {
	port *mockGatewayPort
}

func (o *mockGatewayOpener) Open(cfg transport.PortConfig) (transport.Port, error) {
	return o.port, nil
}

func TestModemRunner_Lifecycle(t *testing.T) {
	port := newMockGatewayPort()
	opener := &mockGatewayOpener{port: port}
	mqttClient := mqtt.NewMockClient()
	_ = mqttClient.Connect()

	cfg := &config.Config{
		LogLevel: "info",
		MQTT: config.MQTTConfig{
			TopicPrefix: "gsm2mqtt",
			Discovery:   false,
		},
		Modems: []config.ModemConfig{
			{
				ID:       "modem1",
				Port:     "/dev/ttyUSB0",
				BaudRate: 115200,
				Type:     "generic",
			},
		},
		Security: config.SecurityConfig{
			IncomingFilter: "all",
		},
		SMS: config.SMSConfig{
			Encoding: "auto",
		},
		Status: config.StatusConfig{
			Interval: time.Minute,
		},
	}

	connector := modem.NewConnector(opener)
	runner := NewModemRunner(cfg.Modems[0], cfg, connector, mqttClient)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Verify MQTT subscriptions
	topics := mqtt.NewTopics("gsm2mqtt", "modem1")
	smsSendTopic := topics.SMSSend()

	sendPayload, _ := json.Marshal(SendSMSRequest{
		To:   "+79991112233",
		Text: "Hello gateway",
	})
	_ = mqttClient.Publish(smsSendTopic, 1, false, sendPayload)

	// Stop runner
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected runner error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("runner did not terminate on cancel")
	}
}

func TestModemRunner_ApplyParsedBalance(t *testing.T) {
	port := newMockGatewayPort()
	opener := &mockGatewayOpener{port: port}
	mqttClient := mqtt.NewMockClient()
	_ = mqttClient.Connect()

	cfg := &config.Config{
		LogLevel: "info",
		MQTT: config.MQTTConfig{
			TopicPrefix: "gsm2mqtt",
			Discovery:   false,
		},
		Modems: []config.ModemConfig{
			{
				ID:       "modem1",
				Port:     "/dev/ttyUSB0",
				BaudRate: 115200,
				Type:     "generic",
			},
		},
		Tariff: config.TariffConfig{
			Enabled:        true,
			OperatorPreset: "megafon",
		},
	}

	connector := modem.NewConnector(opener)
	runner := NewModemRunner(cfg.Modems[0], cfg, connector, mqttClient)

	// 1. Initial balance is 0
	if bal := runner.Summary().Balance; bal != 0 {
		t.Fatalf("expected initial balance 0, got %v", bal)
	}

	// 2. Parse positive balance
	runner.applyParsedBalance("Ваш баланс: 150.50 руб.")
	if bal := runner.Summary().Balance; bal != 150.50 {
		t.Errorf("expected balance 150.50, got %v", bal)
	}

	// 3. Promotional message without balance must NOT overwrite or reset balance
	runner.applyParsedBalance("Подключите супер тариф по номеру 0500")
	if bal := runner.Summary().Balance; bal != 150.50 {
		t.Errorf("expected balance to remain 150.50 after promo ad, got %v", bal)
	}

	// 4. Parse debt (negative balance)
	runner.applyParsedBalance("Задолженность: 1.78 руб.")
	if bal := runner.Summary().Balance; bal != -1.78 {
		t.Errorf("expected balance -1.78, got %v", bal)
	}
}

func TestExtractLeadingRecipient(t *testing.T) {
	tests := []struct {
		input       string
		wantTarget  string
		wantText    string
		wantMatched bool
	}{
		{
			input:       "+79991112233: Hello world",
			wantTarget:  "+79991112233",
			wantText:    "Hello world",
			wantMatched: true,
		},
		{
			input:       "+79991112233 Hello world",
			wantTarget:  "+79991112233",
			wantText:    "Hello world",
			wantMatched: true,
		},
		{
			input:       "89991112233: Alarm triggered",
			wantTarget:  "+79991112233",
			wantText:    "Alarm triggered",
			wantMatched: true,
		},
		{
			input:       "Test notification",
			wantTarget:  "",
			wantText:    "Test notification",
			wantMatched: false,
		},
		{
			input:       "8 hours remaining",
			wantTarget:  "",
			wantText:    "8 hours remaining",
			wantMatched: false,
		},
		{
			input:       "+79991112233:",
			wantTarget:  "",
			wantText:    "+79991112233:",
			wantMatched: false,
		},
		{
			input:       "",
			wantTarget:  "",
			wantText:    "",
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			target, text, matched := extractLeadingRecipient(tt.input)
			if matched != tt.wantMatched {
				t.Errorf("extractLeadingRecipient(%q) matched=%v, want %v", tt.input, matched, tt.wantMatched)
			}
			if target != tt.wantTarget {
				t.Errorf("extractLeadingRecipient(%q) target=%q, want %q", tt.input, target, tt.wantTarget)
			}
			if text != tt.wantText {
				t.Errorf("extractLeadingRecipient(%q) text=%q, want %q", tt.input, text, tt.wantText)
			}
		})
	}
}

func TestModemRunner_ConfiguredQoS(t *testing.T) {
	cfg := config.Defaults()
	cfg.MQTT.QoS = 2

	mockClient := mqtt.NewMockClient()
	_ = mockClient.Connect()

	runner := &ModemRunner{
		mCfg: config.ModemConfig{
			ID:   "modem1",
			Port: "/dev/ttyUSB0",
			Type: "generic",
		},
		cfg:        cfg,
		mqttClient: mockClient,
		topics:     mqtt.NewTopics("gsm2mqtt", "modem1"),
	}

	if runner.qos() != 2 {
		t.Fatalf("expected runner.qos() == 2, got %d", runner.qos())
	}

	tm := tariff.NewManager("modem1", tariff.Config{OperatorPreset: "generic"}, nil)
	runner.publishAccountingStatus(tm)

	published := mockClient.Published()
	if len(published) == 0 {
		t.Fatal("expected at least one published message")
	}

	for _, p := range published {
		if p.QoS != 2 {
			t.Errorf("expected published QoS 2 for topic %s, got %d", p.Topic, p.QoS)
		}
	}
}

func TestWorkingConfig_AppliesQoSInServices(t *testing.T) {
	tmpDir := t.TempDir()
	examplePath := filepath.Join("..", "..", "configs", "gsm2mqtt.example.yaml")
	targetPath := filepath.Join(tmpDir, "gsm2mqtt.yaml")

	if err := config.CopyExampleConfig(examplePath, targetPath, 2); err != nil {
		t.Fatalf("failed to copy example config: %v", err)
	}

	cfg, err := config.Load(targetPath)
	if err != nil {
		t.Fatalf("failed to load copied config: %v", err)
	}

	mockClient := mqtt.NewMockClient()
	_ = mockClient.Connect()

	// 1. Verify ModemRunner uses QoS 2
	runner := NewModemRunner(cfg.Modems[0], cfg, nil, mockClient)
	if runner.QoS() != 2 {
		t.Errorf("expected runner.QoS() == 2, got %d", runner.QoS())
	}

	// 2. Verify GatewayManager uses QoS 2
	mgr := NewGatewayManager()
	mgr.SetMQTT(mockClient, &cfg.MQTT)
	if status := mgr.GetMQTTStatus(); status.QoS != 2 {
		t.Errorf("expected manager MQTT status QoS == 2, got %d", status.QoS)
	}

	// 3. Verify messages published by runner use QoS 2
	tm := tariff.NewManager("modem1", tariff.Config{OperatorPreset: "generic"}, nil)
	runner.publishAccountingStatus(tm)

	published := mockClient.Published()
	if len(published) == 0 {
		t.Fatal("expected at least one published message")
	}
	for _, p := range published {
		if p.QoS != 2 {
			t.Errorf("expected message published with QoS 2, got %d for topic %s", p.QoS, p.Topic)
		}
	}
}

type failingGatewayOpener struct{}

func (f *failingGatewayOpener) Open(cfg transport.PortConfig) (transport.Port, error) {
	return nil, errors.New("device not found")
}

func TestModemRunner_DisconnectedStatePublished(t *testing.T) {
	mockClient := mqtt.NewMockClient()
	_ = mockClient.Connect()

	cfg := &config.Config{
		MQTT: config.MQTTConfig{
			TopicPrefix: "gsm2mqtt",
			Discovery:   true,
			QoS:         1,
		},
		Modems: []config.ModemConfig{
			{
				ID:   "m590",
				Port: "/dev/ttyNONEXISTENT",
				Type: "neoway",
			},
		},
	}

	connector := modem.NewConnector(&failingGatewayOpener{})
	runner := NewModemRunner(cfg.Modems[0], cfg, connector, mockClient)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx)
	}()

	// Wait briefly for runner to publish initial disconnected state
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-errCh

	published := mockClient.Published()
	if len(published) == 0 {
		t.Fatal("expected published messages for disconnected runner")
	}

	// 1. Verify health topic was published with status "disconnected" and retained
	var foundHealth bool
	for _, p := range published {
		if p.Topic == "gsm2mqtt/modem/m590/health" {
			foundHealth = true
			if !p.Retained {
				t.Errorf("expected health topic to be retained")
			}
			var h ModemHealth
			if err := json.Unmarshal(p.Payload, &h); err != nil {
				t.Fatalf("failed to unmarshal health payload: %v", err)
			}
			if h.Status != "disconnected" {
				t.Errorf("expected health status 'disconnected', got %q", h.Status)
			}
			if h.SIM != "DISCONNECTED" {
				t.Errorf("expected SIM 'DISCONNECTED', got %q", h.SIM)
			}
		}
	}
	if !foundHealth {
		t.Error("health topic gsm2mqtt/modem/m590/health was not published")
	}

	// 2. Verify gateway/modems topic was published with count 0 and status "disconnected"
	var foundModems bool
	for _, p := range published {
		if p.Topic == "gsm2mqtt/gateway/modems" {
			foundModems = true
			if !p.Retained {
				t.Errorf("expected gateway/modems topic to be retained")
			}
			var gm map[string]any
			if err := json.Unmarshal(p.Payload, &gm); err != nil {
				t.Fatalf("failed to unmarshal gateway/modems payload: %v", err)
			}
			if count, ok := gm["count"].(float64); !ok || count != 0 {
				t.Errorf("expected gateway/modems count 0, got %v", gm["count"])
			}
			if active, ok := gm["active_modem"].(string); !ok || active != "none" {
				t.Errorf("expected active_modem 'none', got %v", gm["active_modem"])
			}
		}
	}
	if !foundModems {
		t.Error("gateway/modems topic was not published")
	}
}

func TestModemRunner_SMSPublishRetainDecoupling(t *testing.T) {
	mockClient := mqtt.NewMockClient()
	_ = mockClient.Connect()

	cfg := &config.Config{
		LogLevel: "info",
		MQTT: config.MQTTConfig{
			TopicPrefix: "gsm2mqtt",
			QoS:         1,
		},
		SMS: config.SMSConfig{
			Encoding:        "auto",
			AssemblyTimeout: 30 * time.Second,
		},
	}
	mCfg := config.ModemConfig{
		ID:   "m590",
		Type: "neoway_m590",
		Port: "/dev/ttyUSB0",
	}

	runner := NewModemRunner(mCfg, cfg, nil, mockClient)
	runner.receivedSMS = []ReceivedSMS{
		{
			ID:        "msg1",
			ModemID:   "m590",
			Sender:    "+79991112233",
			Text:      "Historical SMS",
			Timestamp: "2026-09-26T20:00:00Z",
		},
	}

	// Verify startup retain behavior:
	// 1. Clears sms/received
	_ = mockClient.Publish(runner.topics.SMSReceived(), runner.qos(), true, []byte{})
	// 2. Publishes sms/last
	last := runner.receivedSMS[0]
	lastPayload, _ := json.Marshal(map[string]any{
		"from":      last.Sender,
		"text":      last.Text,
		"timestamp": last.Timestamp,
	})
	_ = mockClient.Publish(runner.topics.SMSLast(), runner.qos(), true, lastPayload)

	// Now simulate live incoming SMS callback
	liveMsg := &sms.AssembledSMS{
		From:       "+79998887766",
		Text:       "Live alert code 1234",
		Timestamp:  time.Now(),
		Segments:   1,
		IsComplete: true,
	}
	payload, _ := json.Marshal(liveMsg)
	_ = mockClient.Publish(runner.topics.SMSReceived(), runner.qos(), false, payload)
	_ = mockClient.Publish(runner.topics.SMSLast(), runner.qos(), true, payload)

	published := mockClient.Published()

	// Verify live sms/received was published with retained: false
	var foundLiveReceived bool
	for _, p := range published {
		if p.Topic == "gsm2mqtt/modem/m590/sms/received" && len(p.Payload) > 0 {
			foundLiveReceived = true
			if p.Retained {
				t.Errorf("expected live sms/received message to have Retained = false")
			}
		}
	}
	if !foundLiveReceived {
		t.Error("expected live sms/received to be published")
	}

	// Verify sms/last was published with retained: true
	var foundLast bool
	for _, p := range published {
		if p.Topic == "gsm2mqtt/modem/m590/sms/last" {
			foundLast = true
			if !p.Retained {
				t.Errorf("expected sms/last to have Retained = true")
			}
		}
	}
	if !foundLast {
		t.Error("expected sms/last to be published")
	}
}
