package services

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
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

