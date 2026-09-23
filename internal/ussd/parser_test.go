package ussd

import (
	"errors"
	"testing"
)

func TestValidateCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr error
	}{
		{
			name:    "valid standard code *100#",
			code:    "*100#",
			wantErr: nil,
		},
		{
			name:    "valid multi-level code *111*1#",
			code:    "*111*1#",
			wantErr: nil,
		},
		{
			name:    "valid hash code #100#",
			code:    "#100#",
			wantErr: nil,
		},
		{
			name:    "empty code",
			code:    "",
			wantErr: ErrEmptyUSSDCode,
		},
		{
			name:    "missing leading or trailing symbol",
			code:    "100",
			wantErr: ErrInvalidUSSDFormat,
		},
		{
			name:    "missing trailing hash",
			code:    "*100",
			wantErr: ErrInvalidUSSDFormat,
		},
		{
			name:    "contains letters",
			code:    "*100#ABC",
			wantErr: ErrInvalidUSSDFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCode(tt.code)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			} else if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestParseResponse_Positive(t *testing.T) {
	tests := []struct {
		name           string
		urc            string
		expectedMsg    string
		expectedStatus Status
		expectedDCS    int
		expectedAction bool
	}{
		{
			name:           "plain text balance response",
			urc:            `+CUSD: 0,"Your balance is 15.50 USD",15`,
			expectedMsg:    "Your balance is 15.50 USD",
			expectedStatus: StatusCompleted,
			expectedDCS:    15,
			expectedAction: false,
		},
		{
			name:           "interactive menu prompt (action required)",
			urc:            `+CUSD: 1,"1. Tariffs\n2. Services\n0. Exit",15`,
			expectedMsg:    "1. Tariffs\n2. Services\n0. Exit",
			expectedStatus: StatusActionRequired,
			expectedDCS:    15,
			expectedAction: true,
		},
		{
			name: "UCS-2 hex encoded cyrillic response (DCS 72)",
			// "Баланс: 150 руб" in UCS-2 BE hex:
			// Б=0411, а=0430, л=043B, а=0430, н=043D, с=0441, :=003A,  =0020, 1=0031, 5=0035, 0=0030,  =0020, р=0440, у=0443, б=0431
			urc:            `+CUSD: 0,"04110430043B0430043D0441003A00200031003500300020044004430431",72`,
			expectedMsg:    "Баланс: 150 руб",
			expectedStatus: StatusCompleted,
			expectedDCS:    72,
			expectedAction: false,
		},
		{
			name:           "response without DCS parameter",
			urc:            `+CUSD: 0,"Balance: OK"`,
			expectedMsg:    "Balance: OK",
			expectedStatus: StatusCompleted,
			expectedDCS:    15,
			expectedAction: false,
		},
		{
			name:           "Status 2 network terminated with UCS-2 message (Siemens / MegaFon)",
			urc:            `+CUSD: 2,"041204300448002004310430043B0430043D0441003A00200032002E003200320020044004430431002E",72`,
			expectedMsg:    "Ваш баланс: 2.22 руб.",
			expectedStatus: StatusTerminated,
			expectedDCS:    72,
			expectedAction: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParseResponse(tt.urc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Message != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, resp.Message)
			}
			if resp.Status != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, resp.Status)
			}
			if resp.DCS != tt.expectedDCS {
				t.Errorf("expected DCS %d, got %d", tt.expectedDCS, resp.DCS)
			}
			if resp.ActionRequired != tt.expectedAction {
				t.Errorf("expected ActionRequired %v, got %v", tt.expectedAction, resp.ActionRequired)
			}
		})
	}
}

func TestParseResponse_NegativeEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		urc     string
		wantErr error
	}{
		{
			name:    "network terminated session (status 2)",
			urc:     `+CUSD: 2`,
			wantErr: ErrUSSDTerminated,
		},
		{
			name:    "operation not supported (status 4)",
			urc:     `+CUSD: 4`,
			wantErr: ErrUSSDNotSupported,
		},
		{
			name:    "network timeout (status 5)",
			urc:     `+CUSD: 5`,
			wantErr: ErrUSSDTimeout,
		},
		{
			name:    "empty response string",
			urc:     "",
			wantErr: ErrMalformedUSSDResponse,
		},
		{
			name:    "not a +CUSD line",
			urc:     "+CMTI: \"SM\", 1",
			wantErr: ErrMalformedUSSDResponse,
		},
		{
			name:    "invalid status format",
			urc:     `+CUSD: invalid`,
			wantErr: ErrMalformedUSSDResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseResponse(tt.urc)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tt.urc)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
			}
		})
	}
}
