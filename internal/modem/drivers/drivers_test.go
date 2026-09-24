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

func TestSiemensDriver_CNMIFallback(t *testing.T) {
	runner := newMockATRunner()
	// Fail first CNMI candidate to test fallback
	runner.responses["AT+CNMI=2,1,0,2,1"] = &at.Response{Error: true}
	runner.responses["AT+CNMI=2,1,0,0,1"] = &at.Response{OK: true}

	driver := NewSiemensDriver(runner)
	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("Siemens Init with CNMI fallback failed: %v", err)
	}

	foundFallback := false
	for _, cmd := range runner.commands {
		if cmd == "AT+CNMI=2,1,0,0,1" {
			foundFallback = true
			break
		}
	}
	if !foundFallback {
		t.Errorf("expected fallback CNMI command to be issued, got: %v", runner.commands)
	}
}

func TestSIMComDriver_Init(t *testing.T) {
	runner := newMockATRunner()
	driver := NewSIMComDriver(runner)

	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("SIMCom Init error: %v", err)
	}

	expectedCmds := []string{"AT", "AT+CSCLK=0", "AT+CFUN=1", "ATE0", "AT+CMEE=2", "AT+CMGF=0", "AT+CNMI=2,1,0,1,0", "AT+CLIP=1"}
	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range runner.commands {
			if cmd == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command %q to be sent during Init, commands sent: %v", expected, runner.commands)
		}
	}
}

func TestSIMComDriver_BatteryStatus(t *testing.T) {
	tests := []struct {
		name         string
		response     *at.Response
		errExpected  bool
		expectedBatt *modem.BatteryInfo
	}{
		{
			name: "valid not charging",
			response: &at.Response{
				OK:    true,
				Lines: []string{"+CBC: 0,85,4120"},
			},
			errExpected: false,
			expectedBatt: &modem.BatteryInfo{
				Charging:   false,
				Percent:    85,
				Millivolts: 4120,
			},
		},
		{
			name: "valid charging",
			response: &at.Response{
				OK:    true,
				Lines: []string{"+CBC: 1,98,4215"},
			},
			errExpected: false,
			expectedBatt: &modem.BatteryInfo{
				Charging:   true,
				Percent:    98,
				Millivolts: 4215,
			},
		},
		{
			name: "at command error",
			response: &at.Response{
				Error: true,
				Lines: []string{"+CME ERROR: 58"},
			},
			errExpected: true,
		},
		{
			name: "malformed response missing fields",
			response: &at.Response{
				OK:    true,
				Lines: []string{"+CBC: not_a_number"},
			},
			errExpected: true,
		},
		{
			name: "empty response lines",
			response: &at.Response{
				OK:    true,
				Lines: []string{},
			},
			errExpected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newMockATRunner()
			runner.responses["AT+CBC"] = tc.response
			driver := NewSIMComDriver(runner)

			batt, err := driver.BatteryStatus()
			if tc.errExpected {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if batt.Charging != tc.expectedBatt.Charging {
				t.Errorf("expected Charging %v, got %v", tc.expectedBatt.Charging, batt.Charging)
			}
			if batt.Percent != tc.expectedBatt.Percent {
				t.Errorf("expected Percent %d, got %d", tc.expectedBatt.Percent, batt.Percent)
			}
			if batt.Millivolts != tc.expectedBatt.Millivolts {
				t.Errorf("expected Millivolts %d, got %d", tc.expectedBatt.Millivolts, batt.Millivolts)
			}
		})
	}
}

func TestSIMComDriver_Audio(t *testing.T) {
	runner := newMockATRunner()
	driver := NewSIMComDriver(runner)

	// Valid volume
	if err := driver.SetVolume(80); err != nil {
		t.Errorf("unexpected error setting volume: %v", err)
	}
	if len(runner.commands) == 0 || runner.commands[len(runner.commands)-1] != "AT+CLVL=80" {
		t.Errorf("expected AT+CLVL=80 to be sent, got %v", runner.commands)
	}

	// Invalid volume
	if err := driver.SetVolume(-1); err == nil {
		t.Errorf("expected error for negative volume")
	}
	if err := driver.SetVolume(101); err == nil {
		t.Errorf("expected error for volume > 100")
	}

	// Valid mic gain
	if err := driver.SetMicGain(10); err != nil {
		t.Errorf("unexpected error setting mic gain: %v", err)
	}
	if len(runner.commands) == 0 || runner.commands[len(runner.commands)-1] != "AT+CMIC=0,10" {
		t.Errorf("expected AT+CMIC=0,10 to be sent, got %v", runner.commands)
	}

	// Invalid mic gain
	if err := driver.SetMicGain(-1); err == nil {
		t.Errorf("expected error for negative mic gain")
	}
	if err := driver.SetMicGain(16); err == nil {
		t.Errorf("expected error for mic gain > 15")
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

	foundVoice := false
	foundColp := false
	for _, cmd := range runner.commands {
		if cmd == "AT^CVOICE=0" {
			foundVoice = true
		}
		if cmd == "AT+COLP=1" {
			foundColp = true
		}
	}
	if !foundVoice {
		t.Errorf("expected AT^CVOICE=0 to be sent, sent: %v", runner.commands)
	}
	if !foundColp {
		t.Errorf("expected AT+COLP=1 to be sent, sent: %v", runner.commands)
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

func TestHuaweiDriver_Dial_VoiceDisabled(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT^CVOICE?"] = &at.Response{OK: true, Lines: []string{"^CVOICE:1"}}
	driver := NewHuaweiDriver(runner)

	err := driver.Dial("+79991112233")
	if err == nil {
		t.Fatal("expected error when voice is disabled, got nil")
	}
	if !strings.Contains(err.Error(), "voice calls are not supported") {
		t.Errorf("expected voice calls not supported error, got: %v", err)
	}

	// Verify ATD was NOT sent
	for _, cmd := range runner.commands {
		if strings.HasPrefix(cmd, "ATD") {
			t.Errorf("ATD should not have been sent when voice is disabled, sent: %v", runner.commands)
		}
	}
}

func TestHuaweiDriver_Dial_VoiceEnabled(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT^CVOICE?"] = &at.Response{OK: true, Lines: []string{"^CVOICE:0"}}
	driver := NewHuaweiDriver(runner)

	err := driver.Dial("+79991112233")
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}

	foundATD := false
	for _, cmd := range runner.commands {
		if strings.HasPrefix(cmd, "ATD+79991112233;") {
			foundATD = true
		}
	}
	if !foundATD {
		t.Errorf("expected ATD command to be sent when voice is enabled, sent: %v", runner.commands)
	}
}

func TestBaseDriver_SelectStorage(t *testing.T) {
	runner := newMockATRunner()
	runner.responses[`AT+CPMS="ME","ME","ME"`] = &at.Response{
		OK:    true,
		Lines: []string{`+CPMS: 5,23,5,23,5,23`},
	}

	driver := NewGenericDriver(runner)
	st, err := driver.SelectStorage("ME")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Name != "ME" || st.Used != 5 || st.Total != 23 {
		t.Errorf("unexpected storage status: %+v", st)
	}

	// Negative case: modem error
	runner.responses[`AT+CPMS="INVALID","INVALID","INVALID"`] = &at.Response{
		Error: true,
		Lines: []string{`+CMS ERROR: 321`},
	}
	_, err = driver.SelectStorage("INVALID")
	if err == nil {
		t.Fatal("expected error on invalid storage, got nil")
	}
}

func TestBaseDriver_StorageCapacity(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+CPMS?"] = &at.Response{
		OK:    true,
		Lines: []string{`+CPMS: "SM",15,15,"SM",15,15,"SM",15,15`},
	}

	driver := NewGenericDriver(runner)
	st, err := driver.StorageCapacity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Name != "SM" || st.Used != 15 || st.Total != 15 {
		t.Errorf("unexpected capacity: %+v", st)
	}
}

func TestBaseDriver_ListMessages(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+CMGL=4"] = &at.Response{
		OK: true,
		Lines: []string{
			`+CMGL: 0,0,,24`,
			`07919720131111F1040C919701111111F100006290221153252104D4F29C0E`,
			`+CMGL: 1,1,,20`,
			`07919720131111F1040C919701111111F100006290221153252104D4F29C0E`,
		},
	}

	driver := NewGenericDriver(runner)
	msgs, err := driver.ListMessages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Index != 0 || msgs[0].Status != 0 {
		t.Errorf("unexpected message 0: %+v", msgs[0])
	}
	if msgs[1].Index != 1 || msgs[1].Status != 1 {
		t.Errorf("unexpected message 1: %+v", msgs[1])
	}
}

func TestBaseDriver_DeleteMessage(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+CMGD=3"] = &at.Response{OK: true}
	runner.responses["AT+CMGD=99"] = &at.Response{Error: true, Lines: []string{"+CMS ERROR: 321"}}

	driver := NewGenericDriver(runner)
	if err := driver.DeleteMessage(3); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := driver.DeleteMessage(99); err == nil {
		t.Fatal("expected error on invalid index, got nil")
	}
}

func TestNeowayDriver_Init(t *testing.T) {
	runner := newMockATRunner()
	driver := NewNeowayDriver(runner)

	ctx := context.Background()
	if err := driver.Init(ctx); err != nil {
		t.Fatalf("Neoway Init error: %v", err)
	}

	expectedCmds := []string{"AT", "ATE0", "AT+CMEE=2", "AT+CMGF=0", "AT+CNMI=2,1,0,1,0", "AT+CLIP=1", "AT+CSCS=\"IRA\""}
	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range runner.commands {
			if cmd == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command %q during Neoway Init, sent: %v", expected, runner.commands)
		}
	}
	for _, cmd := range runner.commands {
		if cmd == "AT+COLP=1" {
			t.Errorf("did not expect AT+COLP=1 during Neoway Init, found in: %v", runner.commands)
		}
	}
}

func TestNeowayDriver_Dial_CallDrop(t *testing.T) {
	runner := newMockATRunner()
	driver := NewNeowayDriver(runner)

	err := driver.Dial("+79964126670")
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}

	foundDial := false
	for _, cmd := range runner.commands {
		if cmd == "ATD+79964126670;" {
			foundDial = true
			break
		}
	}
	if !foundDial {
		t.Errorf("expected ATD+79964126670; to be sent, sent: %v", runner.commands)
	}

	if err := driver.Hangup(); err != nil {
		t.Fatalf("unexpected hangup error: %v", err)
	}
	if len(runner.commands) == 0 || runner.commands[len(runner.commands)-1] != "ATH" {
		t.Errorf("expected ATH to be sent on hangup, sent: %v", runner.commands)
	}
}

func TestNeowayDriver_BatteryStatus(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+CBC"] = &at.Response{
		OK:    true,
		Lines: []string{"+CBC: 0,90,4050"},
	}
	driver := NewNeowayDriver(runner)

	batt, err := driver.BatteryStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batt.Charging || batt.Percent != 90 || batt.Millivolts != 4050 {
		t.Errorf("unexpected battery status: %+v", batt)
	}
}

func TestNeowayDriver_SendUSSD_UCS2Hex(t *testing.T) {
	runner := newMockATRunner()
	expectedCmd := `AT+CUSD=1,"002A0031003000300023",15`
	runner.responses[expectedCmd] = &at.Response{
		OK:    true,
		Lines: []string{`+CUSD: 0,"041204300448002004310430043B0430043D0441003A00200032002E003200320020044004430431002E",72`},
	}

	driver := NewNeowayDriver(runner)
	resp, err := driver.SendUSSD("*100#")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "+CUSD: 0,") {
		t.Errorf("expected CUSD response line, got %q", resp)
	}

	// Verify that character set was restored to IRA upon exit
	lastCmd := runner.commands[len(runner.commands)-1]
	if lastCmd != `AT+CSCS="IRA"` {
		t.Errorf("expected last command to restore AT+CSCS=\"IRA\", got %q", lastCmd)
	}
}

func TestNeowayDriver_CheckCallState(t *testing.T) {
	runner := newMockATRunner()
	driver := NewNeowayDriver(runner)

	tests := []struct {
		name     string
		response []string
		expected string
	}{
		{
			name:     "dialing",
			response: []string{`+CLCC: 1,0,2,0,0,"89964126670",129`},
			expected: "dialing",
		},
		{
			name:     "ringing",
			response: []string{`+CLCC: 1,0,3,0,0,"89964126670",129`},
			expected: "ringing",
		},
		{
			name:     "answered",
			response: []string{`+CLCC: 1,0,0,0,0,"89964126670",129`},
			expected: "answered",
		},
		{
			name:     "idle",
			response: []string{},
			expected: "idle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner.responses["AT+CLCC"] = &at.Response{
				OK:    true,
				Lines: tt.response,
			}
			state, err := driver.CheckCallState()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if state != tt.expected {
				t.Errorf("expected state %q, got %q", tt.expected, state)
			}
		})
	}
}

func TestSiemensDriver_Identify(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["ATI"] = &at.Response{
		OK:    true,
		Lines: []string{"SIEMENS", "MC35i", "REVISION 02.00"},
	}
	runner.responses["AT+CGSN"] = &at.Response{
		OK:    true,
		Lines: []string{"353857015410240"},
	}
	runner.responses["AT+CIMI"] = &at.Response{
		OK:    true,
		Lines: []string{"250023055574883"},
	}

	driver := NewSiemensDriver(runner)
	info, err := driver.Identify()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Manufacturer != "SIEMENS" {
		t.Errorf("expected manufacturer SIEMENS, got %q", info.Manufacturer)
	}
	if info.Model != "MC35i" {
		t.Errorf("expected model MC35i, got %q", info.Model)
	}
	if info.Revision != "REVISION 02.00" {
		t.Errorf("expected revision 'REVISION 02.00', got %q", info.Revision)
	}
	if info.IMEI != "353857015410240" {
		t.Errorf("expected IMEI '353857015410240', got %q", info.IMEI)
	}
}
