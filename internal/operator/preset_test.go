package operator

import (
	"errors"
	"math"
	"testing"
)

func TestDetectPreset(t *testing.T) {
	tests := []struct {
		name         string
		operatorName string
		wantName     string
		wantUSSD     string
	}{
		{"MTS English", "MTS", "mts", "*100#"},
		{"MTS Cyrillic", "МТС", "mts", "*100#"},
		{"MTS with prefix", "RUS MTS", "mts", "*100#"},
		{"MegaFon English", "MegaFon", "megafon", "*100#"},
		{"MegaFon Cyrillic", "МегаФон", "megafon", "*100#"},
		{"MegaFon RUS", "MegaFon RUS", "megafon", "*100#"},
		{"Beeline English", "Beeline", "beeline", "*102#"},
		{"Beeline Cyrillic", "Билайн", "beeline", "*102#"},
		{"Beeline BeeLine", "BeeLine", "beeline", "*102#"},
		{"Tele2", "Tele2", "tele2", "*105#"},
		{"Tele2 uppercase", "TELE2", "tele2", "*105#"},
		{"T2 Russian", "Т2", "tele2", "*105#"},
		{"Unknown fallback", "Some Unknown Network", "generic", "*100#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preset := DetectPreset(tt.operatorName)
			if preset == nil {
				t.Fatalf("DetectPreset(%q) returned nil", tt.operatorName)
			}
			if preset.Name != tt.wantName {
				t.Errorf("DetectPreset(%q).Name = %q, want %q", tt.operatorName, preset.Name, tt.wantName)
			}
			if preset.USSDCode != tt.wantUSSD {
				t.Errorf("DetectPreset(%q).USSDCode = %q, want %q", tt.operatorName, preset.USSDCode, tt.wantUSSD)
			}
		})
	}
}

func TestGetPreset(t *testing.T) {
	tests := []struct {
		name     string
		preset   string
		wantErr  error
		wantUSSD string
	}{
		{"mts", "mts", nil, "*100#"},
		{"megafon", "megafon", nil, "*100#"},
		{"beeline", "beeline", nil, "*102#"},
		{"tele2", "tele2", nil, "*105#"},
		{"unknown", "unknown_operator", ErrUnknownPreset, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preset, err := GetPreset(tt.preset)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetPreset(%q) error = %v, wantErr = %v", tt.preset, err, tt.wantErr)
			}
			if tt.wantErr == nil && preset.USSDCode != tt.wantUSSD {
				t.Errorf("GetPreset(%q).USSDCode = %q, want %q", tt.preset, preset.USSDCode, tt.wantUSSD)
			}
		})
	}
}

func TestParseBalance(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		regex       string
		wantBalance float64
		wantErr     error
	}{
		{
			name:        "standard mts balance",
			text:        "Баланс: 152.40 руб. Лимит: 0.00 руб.",
			regex:       "",
			wantBalance: 152.40,
			wantErr:     nil,
		},
		{
			name:        "balance with comma delimiter",
			text:        "Баланс: 152,40р.",
			regex:       "",
			wantBalance: 152.40,
			wantErr:     nil,
		},
		{
			name:        "negative balance",
			text:        "Vash balans: -15.50 r.",
			regex:       "",
			wantBalance: -15.50,
			wantErr:     nil,
		},
		{
			name:        "english balance keyword",
			text:        "Balance: 50.00 RUB",
			regex:       "",
			wantBalance: 50.00,
			wantErr:     nil,
		},
		{
			name:        "balance with ruble symbol",
			text:        "Остаток: 250.75 ₽",
			regex:       "",
			wantBalance: 250.75,
			wantErr:     nil,
		},
		{
			name:        "custom regex",
			text:        "Account funds: $99.95 available",
			regex:       `(?i)funds:\s*\$([\d\.,]+)`,
			wantBalance: 99.95,
			wantErr:     nil,
		},
		{
			name:        "empty response text",
			text:        "",
			regex:       "",
			wantBalance: 0,
			wantErr:     ErrEmptyResponse,
		},
		{
			name:        "no balance found",
			text:        "Spasibo, vash platezh prinyat",
			regex:       "",
			wantBalance: 0,
			wantErr:     ErrBalanceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBalance(tt.text, tt.regex)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseBalance(%q) error = %v, wantErr = %v", tt.text, err, tt.wantErr)
			}
			if tt.wantErr == nil && math.Abs(got-tt.wantBalance) > 0.001 {
				t.Errorf("ParseBalance(%q) = %v, want %v", tt.text, got, tt.wantBalance)
			}
		})
	}
}
