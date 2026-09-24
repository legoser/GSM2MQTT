package security

import (
	"errors"
	"testing"
)

func TestSanitizer_RawDisabledByDefault(t *testing.T) {
	s := NewSanitizer(false, []string{"ATI"})

	if s.IsAllowed("ATI") {
		t.Errorf("when allowRaw is false, all raw AT commands must be blocked")
	}
	if s.IsAllowed("AT+CSQ") {
		t.Errorf("when allowRaw is false, even safe query commands must be blocked")
	}
}

func TestSanitizer_AllowedCommands(t *testing.T) {
	allowed := []string{
		"ATI",
		"AT+CSQ",
		"AT+COPS",
	}
	s := NewSanitizer(true, allowed)

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

func TestSanitizer_PrefixTrap(t *testing.T) {
	// Regression test for N1: a broad entry like "AT+C" must NOT authorize
	// "AT+CFUN", "AT+CPIN" or "AT+CMGS". Matching is on token boundaries.
	s := NewSanitizer(true, []string{"ATI", "AT+C"})

	blocked := []string{
		"AT+CFUN=0",
		"at+cfun=0",
		"AT+CFUN=1,1",
		"AT+CPIN?",
		"AT+CPIN=\"1234\"",
		"AT+CMGS=25",
		"AT+CMSS=1",
		"AT+CUSD=1,\"*100#\",15",
		"AT+CGDCONT=1,\"IP\",\"internet\"",
	}
	for _, cmd := range blocked {
		if s.IsAllowed(cmd) {
			t.Errorf("prefix trap: %q must be blocked with allow=[ATI AT+C]", cmd)
		}
		if err := s.Validate(cmd); !errors.Is(err, ErrCommandBlocked) {
			t.Errorf("Validate(%q) = %v, want ErrCommandBlocked", cmd, err)
		}
	}
}

func TestSanitizer_Validate_Table(t *testing.T) {
	defaultAllowed := []string{
		"ATI",
		"AT+CSQ",
		"AT+COPS",
	}

	tests := []struct {
		name     string
		allowRaw bool
		allowed  []string
		cmd      string
		wantErr  error
	}{
		{
			name:     "raw disabled rejects valid command",
			allowRaw: false,
			allowed:  defaultAllowed,
			cmd:      "ATI",
			wantErr:  ErrRawATDisabled,
		},
		{
			name:     "empty command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "",
			wantErr:  ErrEmptyCommand,
		},
		{
			name:     "whitespace command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "   \t  ",
			wantErr:  ErrEmptyCommand,
		},
		{
			name:     "safe command uppercase",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI",
			wantErr:  nil,
		},
		{
			name:     "safe command lowercase",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "at+csq",
			wantErr:  nil,
		},
		{
			name:     "safe command with whitespace padding",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "   AT+COPS?   ",
			wantErr:  nil,
		},
		{
			name:     "safe indexed command ATI0",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI0",
			wantErr:  nil,
		},
		{
			name:     "safe query with args",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "AT+CSQ?",
			wantErr:  nil,
		},
		{
			name:     "blocked AT+CFUN=0 exact",
			allowRaw: true,
			allowed:  []string{"ATI", "AT+CSQ"},
			cmd:      "AT+CFUN=0",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CFUN=0 lowercase",
			allowRaw: true,
			allowed:  []string{"ATI", "AT+CSQ"},
			cmd:      "at+cfun=0",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CFUN = 0 with inner spaces",
			allowRaw: true,
			allowed:  []string{"ATI", "AT+CSQ"},
			cmd:      "AT+CFUN = 0",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CPIN prefix with args",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "AT+CPIN=\"1234\"",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CPIN query",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "AT+CPIN?",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked ATD dial",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATD+79991112233;",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT&F factory reset",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "AT&F",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CGDCONT apn rewrite",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "AT+CGDCONT=1,\"IP\",\"internet\"",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked suffix trick CSQX",
			allowRaw: true,
			allowed:  []string{"ATI", "AT+CSQ"},
			cmd:      "AT+CSQX",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "null byte injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x00AT+CFUN=0",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "ctrl-z injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x1A",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "esc injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x1B",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "bell injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x07",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "vertical tab injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x0B",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "form feed injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x0C",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "del injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\x7F",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "newline injection in command",
			allowRaw: true,
			allowed:  defaultAllowed,
			cmd:      "ATI\r\nAT+CFUN=0",
			wantErr:  ErrDangerousChars,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSanitizer(tt.allowRaw, tt.allowed)
			err := s.Validate(tt.cmd)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate(%q) error = %v, wantErr = %v", tt.cmd, err, tt.wantErr)
			}
			if tt.wantErr == nil && !s.IsAllowed(tt.cmd) {
				t.Errorf("IsAllowed(%q) = false, want true", tt.cmd)
			}
			if tt.wantErr != nil && s.IsAllowed(tt.cmd) {
				t.Errorf("IsAllowed(%q) = true, want false", tt.cmd)
			}
		})
	}
}
