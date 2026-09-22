package security

import (
	"errors"
	"testing"
)

func TestFilter_All(t *testing.T) {
	f := NewFilter("all", nil, nil)
	if !f.Allowed("+79991112233") {
		t.Errorf("expected number to be allowed in 'all' mode")
	}
}

func TestFilter_Whitelist(t *testing.T) {
	f := NewFilter("whitelist", []string{"+79991112233"}, nil)

	if !f.Allowed("+79991112233") {
		t.Errorf("expected whitelisted number to be allowed")
	}
	if f.Allowed("+79999999999") {
		t.Errorf("expected non-whitelisted number to be blocked")
	}
}

func TestFilter_Blacklist(t *testing.T) {
	f := NewFilter("blacklist", nil, []string{"+79998887766"})

	if f.Allowed("+79998887766") {
		t.Errorf("expected blacklisted number to be blocked")
	}
	if !f.Allowed("+79991112233") {
		t.Errorf("expected non-blacklisted number to be allowed")
	}
}

func TestFilter_Check_Table(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		whitelist []string
		blacklist []string
		number    string
		wantErr   error
	}{
		{
			name:    "all mode allows any valid number",
			mode:    "all",
			number:  "+79991234567",
			wantErr: nil,
		},
		{
			name:    "all mode uppercase",
			mode:    "ALL",
			number:  "+79991234567",
			wantErr: nil,
		},
		{
			name:    "all mode allows alphanumeric sender",
			mode:    "all",
			number:  "MCHS",
			wantErr: nil,
		},
		{
			name:      "whitelist mode allows matched number",
			mode:      "whitelist",
			whitelist: []string{"+79991112233"},
			number:    "+79991112233",
			wantErr:   nil,
		},
		{
			name:      "whitelist mode allows formatted number match",
			mode:      "whitelist",
			whitelist: []string{"+79991112233"},
			number:    "+7 (999) 111-22-33",
			wantErr:   nil,
		},
		{
			name:      "whitelist mode allows alphanumeric match case-insensitive",
			mode:      "whitelist",
			whitelist: []string{"Google"},
			number:    "GOOGLE",
			wantErr:   nil,
		},
		{
			name:      "whitelist mode rejects unlisted number",
			mode:      "whitelist",
			whitelist: []string{"+79991112233"},
			number:    "+79990000000",
			wantErr:   ErrNumberNotWhitelisted,
		},
		{
			name:      "blacklist mode allows unlisted number",
			mode:      "blacklist",
			blacklist: []string{"+79998887766"},
			number:    "+79991112233",
			wantErr:   nil,
		},
		{
			name:      "blacklist mode rejects blacklisted number",
			mode:      "blacklist",
			blacklist: []string{"+79998887766"},
			number:    "+79998887766",
			wantErr:   ErrNumberBlocked,
		},
		{
			name:      "blacklist mode rejects formatted blacklisted number",
			mode:      "blacklist",
			blacklist: []string{"+79998887766"},
			number:    "+7 (999) 888-77-66",
			wantErr:   ErrNumberBlocked,
		},
		{
			name:    "invalid filter mode",
			mode:    "unsupported_mode",
			number:  "+79991112233",
			wantErr: ErrInvalidFilterMode,
		},
		{
			name:    "empty phone number",
			mode:    "all",
			number:  "",
			wantErr: ErrEmptyPhoneNumber,
		},
		{
			name:    "whitespace phone number",
			mode:    "all",
			number:  "   ",
			wantErr: ErrEmptyPhoneNumber,
		},
		{
			name:    "null byte injection in number",
			mode:    "all",
			number:  "+7999\x001112233",
			wantErr: ErrDangerousChars,
		},
		{
			name:    "newline injection in number",
			mode:    "all",
			number:  "+79991112233\r\n",
			wantErr: ErrDangerousChars,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.mode, tt.whitelist, tt.blacklist)
			err := f.Check(tt.number)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Check(%q) error = %v, wantErr = %v", tt.number, err, tt.wantErr)
			}
			if tt.wantErr == nil && !f.Allowed(tt.number) {
				t.Errorf("Allowed(%q) = false, want true", tt.number)
			}
			if tt.wantErr != nil && f.Allowed(tt.number) {
				t.Errorf("Allowed(%q) = true, want false", tt.number)
			}
		})
	}
}
