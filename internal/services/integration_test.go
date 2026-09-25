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

// interactiveModemPort simulates a realistic bidirectional AT command interface
// responding to standard initialization, SMS PDU prompts, and USSD dialogues.
type interactiveModemPort struct {
	mu      sync.Mutex
	cond    *sync.Cond
	buf     []byte
	closed  bool
	history []string
}

func newInteractiveModemPort() *interactiveModemPort {
	p := &interactiveModemPort{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *interactiveModemPort) Read(b []byte) (int, error) {
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

func (p *interactiveModemPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, io.EOF
	}

	cmd := string(b)
	p.history = append(p.history, cmd)

	var response string
	switch {
	case strings.HasPrefix(cmd, "AT+CMGS="):
		// PDU header command: modem responds with '>' prompt without CRLF
		response = "\r\n> "
	case strings.Contains(cmd, "\x1A"):
		// PDU payload terminated by Ctrl-Z (\x1A): modem acknowledges with +CMGS and OK
		response = "\r\n+CMGS: 42\r\n\r\nOK\r\n"
	case strings.Contains(cmd, "AT+CUSD"):
		// USSD balance query
		response = "\r\nOK\r\n\r\n+CUSD: 0, \"Balance: 250.75 RUB\", 15\r\n"
	case strings.HasPrefix(cmd, "AT+CSQ"):
		response = "\r\n+CSQ: 24,99\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+COPS?"):
		response = "\r\n+COPS: 0,0,\"MegaFon\"\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+CREG?"):
		response = "\r\n+CREG: 0,1\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+CGMI"):
		response = "\r\nGenericModem\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+CGMM"):
		response = "\r\nSIM800\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+CGMR"):
		response = "\r\nR14.18\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+CGSN"):
		response = "\r\n860000000000001\r\n\r\nOK\r\n"
	case strings.HasPrefix(cmd, "AT+CIMI"):
		response = "\r\n250990000000001\r\n\r\nOK\r\n"
	default:
		// Default response for ATE0, AT+CMEE=2, AT+CMGF=0, AT+CNMI, etc.
		response = "\r\nOK\r\n"
	}

	p.buf = append(p.buf, []byte(response)...)
	p.cond.Broadcast()
	return len(b), nil
}

func (p *interactiveModemPort) InjectURC(urc string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, []byte("\r\n"+urc+"\r\n")...)
	p.cond.Broadcast()
}

func (p *interactiveModemPort) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.cond.Broadcast()
	return nil
}

func (p *interactiveModemPort) SetDTR(bool) error        { return nil }
func (p *interactiveModemPort) SetRTS(bool) error        { return nil }
func (p *interactiveModemPort) ResetInputBuffer() error  { return nil }
func (p *interactiveModemPort) ResetOutputBuffer() error { return nil }

type interactiveModemOpener struct {
	port *interactiveModemPort
}

func (o *interactiveModemOpener) Open(_ transport.PortConfig) (transport.Port, error) {
	return o.port, nil
}

// TestIntegration_GatewayPipeline verifies the end-to-end processing pipeline:
// 1. Modem runner startup and hardware initialization.
// 2. Outgoing SMS request from MQTT -> PDU encoder -> AT+CMGS dialogue -> tariff accounting update.
// 3. USSD query from MQTT -> AT+CUSD dialogue -> balance parsing -> MQTT balance publication.
// 4. Clean and deadlock-free service shutdown on context cancellation.
func TestIntegration_GatewayPipeline(t *testing.T) {
	port := newInteractiveModemPort()
	opener := &interactiveModemOpener{port: port}
	mqttClient := mqtt.NewMockClient()
	if err := mqttClient.Connect(); err != nil {
		t.Fatalf("failed to connect mock MQTT client: %v", err)
	}

	cfg := &config.Config{
		LogLevel: "debug",
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
		Tariff: config.TariffConfig{
			Enabled:        true,
			OperatorPreset: "megafon",
			SMSLimit:       100,
			CheckInterval:  0, // Disable automatic periodic checks during test
		},
		Status: config.StatusConfig{
			Interval: time.Hour,
		},
	}

	connector := modem.NewConnector(opener)
	runner := NewModemRunner(cfg.Modems[0], cfg, connector, mqttClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runnerErrCh := make(chan error, 1)
	go func() {
		runnerErrCh <- runner.Run(ctx)
	}()

	// Wait briefly for runner to initialize driver and subscriptions
	time.Sleep(100 * time.Millisecond)

	topics := mqtt.NewTopics("gsm2mqtt", "modem1")

	// ---------------------------------------------------------
	// 1. E2E Outgoing SMS Pipeline Test
	// ---------------------------------------------------------
	t.Run("outgoing SMS pipeline and tariff tracking", func(t *testing.T) {
		tariffStatusCh := make(chan tariff.UsageStatus, 1)
		err := mqttClient.Subscribe(topics.AccountingStatus(), 1, func(_ string, payload []byte) {
			var st tariff.UsageStatus
			if err := json.Unmarshal(payload, &st); err == nil {
				select {
				case tariffStatusCh <- st:
				default:
				}
			}
		})
		if err != nil {
			t.Fatalf("failed to subscribe to accounting status: %v", err)
		}

		sendReq := SendSMSRequest{
			To:   "+79991234567",
			Text: "Hello Integration Test!",
		}
		reqPayload, err := json.Marshal(sendReq)
		if err != nil {
			t.Fatalf("failed to marshal SMS request: %v", err)
		}

		// Publish outgoing SMS command via MQTT
		if err := mqttClient.Publish(topics.SMSSend(), 1, false, reqPayload); err != nil {
			t.Fatalf("failed to publish SMS send command: %v", err)
		}

		// Await updated accounting status showing incremented SMS count
		select {
		case st := <-tariffStatusCh:
			if st.SMSMonthCount != 1 {
				t.Errorf("expected SMSMonthCount = 1 after send, got %d", st.SMSMonthCount)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for accounting status update after SMS send")
		}

		// Verify AT command history recorded AT+CMGS and Ctrl-Z PDU payload
		port.mu.Lock()
		history := make([]string, len(port.history))
		copy(history, port.history)
		port.mu.Unlock()

		hasCMGS := false
		hasCtrlZ := false
		for _, h := range history {
			if strings.HasPrefix(h, "AT+CMGS=") {
				hasCMGS = true
			}
			if strings.Contains(h, "\x1A") {
				hasCtrlZ = true
			}
		}
		if !hasCMGS {
			t.Error("expected port history to contain AT+CMGS command")
		}
		if !hasCtrlZ {
			t.Error("expected port history to contain Ctrl-Z PDU payload")
		}
	})

	// ---------------------------------------------------------
	// 2. E2E USSD Balance Query Pipeline Test
	// ---------------------------------------------------------
	t.Run("USSD query and balance update pipeline", func(t *testing.T) {
		balanceCh := make(chan string, 1)
		err := mqttClient.Subscribe(topics.Balance(), 1, func(_ string, payload []byte) {
			select {
			case balanceCh <- string(payload):
			default:
			}
		})
		if err != nil {
			t.Fatalf("failed to subscribe to balance topic: %v", err)
		}

		ussdReq := struct {
			Code string `json:"code"`
		}{
			Code: "*100#",
		}
		ussdPayload, _ := json.Marshal(ussdReq)

		// Publish USSD request via MQTT
		if err := mqttClient.Publish(topics.USSDSend(), 1, false, ussdPayload); err != nil {
			t.Fatalf("failed to publish USSD command: %v", err)
		}

		// Await balance update published to MQTT
		select {
		case bal := <-balanceCh:
			if bal != "250.75" {
				t.Errorf("expected balance '250.75', got %q", bal)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for balance update after USSD query")
		}

		// Verify live summary state in runner
		summary := runner.Summary()
		if summary.Balance != 250.75 {
			t.Errorf("expected runner summary balance = 250.75, got %v", summary.Balance)
		}
	})

	// ---------------------------------------------------------
	// 3. E2E Incoming SMS (URC) Pipeline Test
	// ---------------------------------------------------------
	t.Run("incoming SMS URC reception and MQTT dispatch", func(t *testing.T) {
		smsRecvCh := make(chan []byte, 1)
		err := mqttClient.Subscribe(topics.SMSReceived(), 1, func(_ string, payload []byte) {
			select {
			case smsRecvCh <- payload:
			default:
			}
		})
		if err != nil {
			t.Fatalf("failed to subscribe to SMS received topic: %v", err)
		}

		// Inject unsolicited CMT notification followed by PDU ("hello" from +7921436587)
		port.InjectURC("+CMT: ,25")
		time.Sleep(10 * time.Millisecond)
		port.InjectURC("00040B919712345678F900006290229000002305C8329BFD0E")

		// Await incoming SMS published to MQTT
		select {
		case payload := <-smsRecvCh:
			var msg struct {
				Sender string `json:"from"`
				Text   string `json:"text"`
			}
			if err := json.Unmarshal(payload, &msg); err != nil {
				t.Fatalf("failed to unmarshal received SMS JSON: %v", err)
			}
			if msg.Text != "Hello" {
				t.Errorf("expected received SMS text 'Hello', got %q", msg.Text)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for incoming SMS to be published to MQTT")
		}
	})

	// ---------------------------------------------------------
	// 4. E2E Clean Shutdown Test (deadlock & goroutine leak guard)
	// ---------------------------------------------------------
	t.Run("graceful termination without deadlocks", func(t *testing.T) {
		cancel()

		select {
		case err := <-runnerErrCh:
			if err != nil && err != context.Canceled {
				t.Fatalf("runner exited with unexpected error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("runner failed to terminate cleanly on context cancellation (possible deadlock)")
		}
	})
}
