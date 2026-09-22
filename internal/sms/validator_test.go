package sms

import (
	"errors"
	"testing"
)

func TestNormalizePhoneNumber_Positive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard E.164",
			input:    "+79001234567",
			expected: "+79001234567",
		},
		{
			name:     "formatted E.164 with spaces and dashes",
			input:    "+7 (900) 123-45-67",
			expected: "+79001234567",
		},
		{
			name:     "national format with 8 replaces with +7",
			input:    "89001234567",
			expected: "+79001234567",
		},
		{
			name:     "formatted national with 8",
			input:    "8 (900) 123-45-67",
			expected: "+79001234567",
		},
		{
			name:     "international number other country (e.g. Germany +49)",
			input:    "+49 151 12345678",
			expected: "+4915112345678",
		},
		{
			name:     "short code (e.g. bank or emergency 900, 112)",
			input:    "900",
			expected: "900",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := NormalizePhoneNumber(tt.input, "+7")
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if actual != tt.expected {
				t.Errorf("NormalizePhoneNumber(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizePhoneNumber_NegativeEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: ErrEmptyPhoneNumber,
		},
		{
			name:    "only spaces",
			input:   "    ",
			wantErr: ErrEmptyPhoneNumber,
		},
		{
			name:    "letters in number",
			input:   "+7900CALLME",
			wantErr: ErrInvalidPhoneCharacters,
		},
		{
			name:    "special characters @#$",
			input:   "+7(900)123@#$",
			wantErr: ErrInvalidPhoneCharacters,
		},
		{
			name:    "too short single digit",
			input:   "1",
			wantErr: ErrPhoneNumberTooShort,
		},
		{
			name:    "too long international (> 15 digits)",
			input:   "+1234567890123456789", // 19 digits
			wantErr: ErrPhoneNumberTooLong,
		},
		{
			name:    "plus in the middle",
			input:   "7900+1234567",
			wantErr: ErrInvalidPhoneCharacters,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizePhoneNumber(tt.input, "+7")
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tt.input)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateMessageText_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr error
	}{
		{
			name:    "valid text",
			text:    "Alert: Temperature is 25°C",
			wantErr: nil,
		},
		{
			name:    "empty text",
			text:    "",
			wantErr: ErrEmptyText,
		},
		{
			name:    "contains null byte",
			text:    "Hello\x00World",
			wantErr: ErrNullByteInText,
		},
		{
			name:    "contains Ctrl+Z (0x1A)",
			text:    "Dangerous\x1Atext",
			wantErr: ErrModemEscapeChar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageText(tt.text)
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			} else if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
			}
		})
	}
}
