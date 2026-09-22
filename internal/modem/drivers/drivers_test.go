package drivers

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
)

type mockATRunner struct {
	mu        sync.Mutex
	commands  []string
	responses map[string]*at.Response
}

func newMockATRunner() *mockATRunner {
	return &mockATRunner{
		responses: make(map[string]*at.Response),
	}
}

func (m *mockATRunner) Send(cmd string, timeout time.Duration) (*at.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleanCmd := strings.TrimSpace(cmd)
	m.commands = append(m.commands, cleanCmd)

	if resp, ok := m.responses[cleanCmd]; ok {
		return resp, nil
	}
	return &at.Response{OK: true}, nil
}

func (m *mockATRunner) SendCommand(ctx context.Context, cmd string) (string, error) {
	resp, err := m.Send(cmd, time.Second)
	if err != nil {
		return "", err
	}
	return strings.Join(resp.Lines, "\n"), nil
}

func TestGenericDriver_Init(t *testing.T) {
	runner := newMockATRunner()
	driver := NewGenericDriver(runner)

	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("unexpected Init error: %v", err)
	}

	expectedCmds := []string{"ATE0", "AT+CMEE=2", "AT+CMGF=0"}
	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range runner.commands {
			if strings.HasPrefix(cmd, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected init command %q to be sent, sent commands: %v", expected, runner.commands)
		}
	}
}

func TestGenericDriver_Caller(t *testing.T) {
	runner := newMockATRunner()
	driver := NewGenericDriver(runner)

	// Dial
	if err := driver.Dial("+79991112233"); err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	if len(runner.commands) == 0 || runner.commands[len(runner.commands)-1] != "ATD+79991112233;" {
		t.Errorf("expected ATD command, got %v", runner.commands)
	}

	// Answer
	if err := driver.Answer(); err != nil {
		t.Fatalf("Answer error: %v", err)
	}
	if runner.commands[len(runner.commands)-1] != "ATA" {
		t.Errorf("expected ATA command, got %v", runner.commands)
	}

	// Hangup
	if err := driver.Hangup(); err != nil {
		t.Fatalf("Hangup error: %v", err)
	}
	if runner.commands[len(runner.commands)-1] != "ATH" {
		t.Errorf("expected ATH command, got %v", runner.commands)
	}

	// DTMF
	if err := driver.SendDTMF("7"); err != nil {
		t.Fatalf("SendDTMF error: %v", err)
	}
	if runner.commands[len(runner.commands)-1] != "AT+VTS=7" {
		t.Errorf("expected AT+VTS=7 command, got %v", runner.commands)
	}
}

func TestGenericDriver_StatusProvider(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+CSQ"] = &at.Response{
		OK:    true,
		Lines: []string{"+CSQ: 20,99"},
	}
	runner.responses["AT+CREG?"] = &at.Response{
		OK:    true,
		Lines: []string{"+CREG: 0,1"},
	}
	runner.responses["AT+COPS?"] = &at.Response{
		OK:    true,
		Lines: []string{`+COPS: 0,0,"MTS",0`},
	}
	runner.responses["AT+CPIN?"] = &at.Response{
		OK:    true,
		Lines: []string{"+CPIN: READY"},
	}

	driver := NewGenericDriver(runner)

	// Signal
	rssi, err := driver.SignalQuality()
	if err != nil {
		t.Fatalf("SignalQuality error: %v", err)
	}
	if rssi != 20 {
		t.Errorf("expected RSSI 20, got %d", rssi)
	}

	// Registration
	reg, err := driver.NetworkRegistration()
	if err != nil {
		t.Fatalf("NetworkRegistration error: %v", err)
	}
	if !reg.Registered || reg.Roaming {
		t.Errorf("expected Registered=true, Roaming=false, got %+v", reg)
	}

	// Operator
	op, err := driver.OperatorName()
	if err != nil {
		t.Fatalf("OperatorName error: %v", err)
	}
	if op != "MTS" {
		t.Errorf("expected operator 'MTS', got %q", op)
	}

	// SIM status
	sim, err := driver.SIMStatus()
	if err != nil {
		t.Fatalf("SIMStatus error: %v", err)
	}
	if sim != modem.SIMReady {
		t.Errorf("expected SIMReady, got %v", sim)
	}
}

func TestGenericDriver_Identify(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+CGMI"] = &at.Response{OK: true, Lines: []string{"GenericCorp"}}
	runner.responses["AT+CGMM"] = &at.Response{OK: true, Lines: []string{"Modem3000"}}
	runner.responses["AT+CGMR"] = &at.Response{OK: true, Lines: []string{"v1.2.3"}}
	runner.responses["AT+CGSN"] = &at.Response{OK: true, Lines: []string{"123456789012345"}}
	runner.responses["AT+CIMI"] = &at.Response{OK: true, Lines: []string{"250011234567890"}}

	driver := NewGenericDriver(runner)
	info, err := driver.Identify()
	if err != nil {
		t.Fatalf("Identify error: %v", err)
	}
	if info.Manufacturer != "GenericCorp" || info.Model != "Modem3000" {
		t.Errorf("unexpected Info: %+v", info)
	}
}

func TestSiemensDriver_AutoBaudInit(t *testing.T) {
	runner := newMockATRunner()
	driver := NewSiemensDriver(runner)

	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("Siemens Init error: %v", err)
	}

	// Must start with "AT" ping for autobaud
	if len(runner.commands) == 0 || runner.commands[0] != "AT" {
		t.Errorf("expected initial 'AT' ping for autobaud sync, got %v", runner.commands)
	}
}

func TestSIMComDriver_Init(t *testing.T) {
	runner := newMockATRunner()
	driver := NewSIMComDriver(runner)

	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("SIMCom Init error: %v", err)
	}
	if len(runner.commands) == 0 {
		t.Fatal("expected commands to be sent")
	}
}

func TestHuaweiDriver_Init(t *testing.T) {
	runner := newMockATRunner()
	driver := NewHuaweiDriver(runner)

	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("Huawei Init error: %v", err)
	}
	if len(runner.commands) == 0 {
		t.Fatal("expected commands to be sent")
	}
}

func TestHuaweiDriver_SendUSSD_FallbackToPDU(t *testing.T) {
	runner := newMockATRunner()
	// Simulate modem rejecting plain text USSD
	runner.responses[`AT+CUSD=1,"*100#",15`] = &at.Response{Error: true, Lines: []string{"ERROR"}}
	runner.responses[`AT+CUSD=1,"*100#"`] = &at.Response{Error: true, Lines: []string{"ERROR"}}
	// Modem accepts 7-bit PDU encoded *100# -> AA180C3602
	runner.responses[`AT+CUSD=1,"AA180C3602",15`] = &at.Response{OK: true, Lines: []string{"OK"}}

	driver := NewHuaweiDriver(runner)
	out, err := driver.SendUSSD("*100#")
	if err != nil {
		t.Fatalf("expected SendUSSD to succeed with PDU fallback, got err: %v", err)
	}
	if out != "OK" {
		t.Errorf("expected OK, got %q", out)
	}
}

