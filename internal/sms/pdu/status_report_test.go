package pdu

import "testing"

func TestDecodeStatusReport_Delivered(t *testing.T) {
	// Standard GSM SMS-STATUS-REPORT PDU:
	// SCA: 00 (no SMSC)
	// FO: 02 (SMS-STATUS-REPORT)
	// TP-MR: 2A (42)
	// TP-RA: 0B 91 97 12 34 56 78 F9 (length 11, intl, +7921436587)
	// TP-SCTS: 62 90 22 90 00 00 23
	// TP-DT: 62 90 22 90 00 15 23
	// TP-ST: 00 (Delivered)
	pduHex := "00022A0B919712345678F9629022900000236290229000152300"

	report, err := DecodeStatusReport(pduHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.MessageRef != 42 {
		t.Errorf("expected MessageRef 42, got %d", report.MessageRef)
	}
	if report.StatusCode != 0x00 {
		t.Errorf("expected StatusCode 0x00, got 0x%02X", report.StatusCode)
	}
	if !report.Delivered {
		t.Errorf("expected Delivered to be true for status code 0x00")
	}
}
