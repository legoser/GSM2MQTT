package security

import "testing"

func TestSanitizer_RawDisabledByDefault(t *testing.T) {
	s := NewSanitizer(false, []string{"AT+CFUN=0"})

	if s.IsAllowed("ATI") {
		t.Errorf("when allowRaw is false, all raw AT commands must be blocked")
	}
	if s.IsAllowed("AT+CSQ") {
		t.Errorf("when allowRaw is false, even safe query commands must be blocked")
	}
}

func TestSanitizer_BlockedCommands(t *testing.T) {
	blocked := []string{
		"AT+CFUN=0",
		"AT+CPIN",
		"ATD",
		"AT&F",
	}
	s := NewSanitizer(true, blocked)

	// Safe commands
	if !s.IsAllowed("ATI") {
		t.Errorf("expected 'ATI' to be allowed")
	}
	if !s.IsAllowed("AT+CSQ") {
		t.Errorf("expected 'AT+CSQ' to be allowed")
	}
	if !s.IsAllowed("AT+COPS?") {
		t.Errorf("expected 'AT+COPS?' to be allowed")
	}

	// Destructive or bypass commands
	if s.IsAllowed("AT+CFUN=0") {
		t.Errorf("expected 'AT+CFUN=0' to be blocked")
	}
	if s.IsAllowed("AT+CPIN=\"1234\"") {
		t.Errorf("expected 'AT+CPIN' to be blocked")
	}
	if s.IsAllowed("ATD+79991112233;") {
		t.Errorf("expected 'ATD' to be blocked")
	}
	if s.IsAllowed("AT&F") {
		t.Errorf("expected 'AT&F' to be blocked")
	}
}
