package pdu

import (
	"errors"
	"testing"
)

func TestEncodeSMS_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
		text      string
		wantErr   error
	}{
		{
			name:      "empty recipient",
			recipient: "",
			text:      "Hello",
			wantErr:   ErrEmptyRecipient,
		},
		{
			name:      "whitespace recipient",
			recipient: "   ",
			text:      "Hello",
			wantErr:   ErrEmptyRecipient,
		},
		{
			name:      "empty text",
			recipient: "+79991112233",
			text:      "",
			wantErr:   ErrEmptyText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EncodeSMS(tt.recipient, tt.text, EncodingAuto, false)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDecodeSMS_NegativeEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pduHex  string
		wantErr error
	}{
		{
			name:    "invalid hex characters",
			pduHex:  "00ZZZZZZZZZZZZZZZZZZZZ",
			wantErr: ErrInvalidHex,
		},
		{
			name:    "odd length hex string",
			pduHex:  "00040",
			wantErr: ErrInvalidHex,
		},
		{
			name:    "too short PDU",
			pduHex:  "0004",
			wantErr: ErrTruncatedPDU,
		},
		{
			name:    "truncated after claimed SCA length",
			pduHex:  "05910000",
			wantErr: ErrTruncatedPDU,
		},
		{
			name:    "truncated in OA digits",
			pduHex:  "00040B919712",
			wantErr: ErrTruncatedPDU,
		},
		{
			name:    "truncated before SCTS timestamp",
			pduHex:  "00040B919712345678F90000",
			wantErr: ErrTruncatedPDU,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeSMS(tt.pduHex)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDecodeStatusReport_NegativeEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pduHex  string
		wantErr error
	}{
		{
			name:    "invalid hex",
			pduHex:  "NOT_HEX_STRING!!",
			wantErr: ErrInvalidHex,
		},
		{
			name:    "too short",
			pduHex:  "0002",
			wantErr: ErrTruncatedPDU,
		},
		{
			name:    "wrong MTI - SMS-DELIVER instead of STATUS-REPORT",
			pduHex:  "00002A0B919712345678F9629022900000236290229000152300",
			wantErr: ErrNotStatusReport,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeStatusReport(tt.pduHex)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
			}
		})
	}
}
