package services

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
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
	// Echo OK for standard commands
	p.buf = append(p.buf, []byte("\r\nOK\r\n")...)
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

	runner := NewModemRunner(cfg.Modems[0], cfg, opener, mqttClient)

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

	runner := NewModemRunner(cfg.Modems[0], cfg, opener, mqttClient)

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

