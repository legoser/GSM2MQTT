package security

import (
	"errors"
	"testing"
)

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

func TestSanitizer_Validate_Table(t *testing.T) {
	defaultBlocked := []string{
		"AT+CFUN=0",
		"AT+CPIN",
		"ATD",
		"AT&F",
		"AT+CGDCONT",
	}

	tests := []struct {
		name     string
		allowRaw bool
		blocked  []string
		cmd      string
		wantErr  error
	}{
		{
			name:     "raw disabled rejects valid command",
			allowRaw: false,
			blocked:  defaultBlocked,
			cmd:      "ATI",
			wantErr:  ErrRawATDisabled,
		},
		{
			name:     "empty command",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "",
			wantErr:  ErrEmptyCommand,
		},
		{
			name:     "whitespace command",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "   \t  ",
			wantErr:  ErrEmptyCommand,
		},
		{
			name:     "safe command uppercase",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "ATI",
			wantErr:  nil,
		},
		{
			name:     "safe command lowercase",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "at+csq",
			wantErr:  nil,
		},
		{
			name:     "safe command with whitespace padding",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "   AT+COPS?   ",
			wantErr:  nil,
		},
		{
			name:     "blocked AT+CFUN=0 exact",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "AT+CFUN=0",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CFUN=0 lowercase",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "at+cfun=0",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CFUN = 0 with inner spaces",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "AT+CFUN = 0",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CPIN prefix with args",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "AT+CPIN=\"1234\"",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CPIN query",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "AT+CPIN?",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked ATD dial",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "ATD+79991112233;",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT&F factory reset",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "AT&F",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "blocked AT+CGDCONT apn rewrite",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "AT+CGDCONT=1,\"IP\",\"internet\"",
			wantErr:  ErrCommandBlocked,
		},
		{
			name:     "null byte injection in command",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "ATI\x00AT+CFUN=0",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "ctrl-z injection in command",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "ATI\x1A",
			wantErr:  ErrDangerousChars,
		},
		{
			name:     "newline injection in command",
			allowRaw: true,
			blocked:  defaultBlocked,
			cmd:      "ATI\r\nAT+CFUN=0",
			wantErr:  ErrDangerousChars,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSanitizer(tt.allowRaw, tt.blocked)
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
