package pdu

import "testing"

func TestDecodeSMS_GSM7(t *testing.T) {
	// SMS-DELIVER with "hello" in GSM 7-bit packing
	pduHex := "00040B919712345678F900006290229000002305C8329BFD0E"

	sms, err := DecodeSMS(pduHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sms == nil {
		t.Fatal("expected non-nil decoded SMS")
	}

	if sms.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", sms.Text)
	}
	if sms.From != "+7921436587" {
		t.Errorf("expected sender '+7921436587', got %q", sms.From)
	}
	if sms.Encoding != EncodingGSM7 {
		t.Errorf("expected EncodingGSM7, got %v", sms.Encoding)
	}
	if sms.HasUDH {
		t.Errorf("expected HasUDH = false")
	}
}

func TestDecodeSMS_UCS2_Cyrillic(t *testing.T) {
	// SMS-DELIVER with "Привет" in UCS-2
	pduHex := "00040B919712345678F90008629022900000230C041F04400438043204350442"

	sms, err := DecodeSMS(pduHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sms == nil {
		t.Fatal("expected non-nil decoded SMS")
	}

	if sms.Text != "Привет" {
		t.Errorf("expected text 'Привет', got %q", sms.Text)
	}
	if sms.Encoding != EncodingUCS2 {
		t.Errorf("expected EncodingUCS2, got %v", sms.Encoding)
	}
}

func TestDecodeSMS_MultipartUDH(t *testing.T) {
	// SMS-DELIVER with UDHI bit set (0x44), UDH = 05 00 03 A7 02 01 (ref 0xA7, 2 parts, part 1)
	// Followed by UCS-2 "Тест" = 0422 0435 0441 0442 (8 bytes)
	// Total UDL = 6 bytes UDH + 8 bytes payload = 14 (0x0E)
	pduHex := "00440B919712345678F90008629022900000230E050003A702010422043504410442"

	sms, err := DecodeSMS(pduHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sms == nil {
		t.Fatal("expected non-nil decoded SMS")
	}

	if !sms.HasUDH {
		t.Errorf("expected HasUDH = true")
	}
	if sms.Reference != 0xA7 {
		t.Errorf("expected reference 0xA7, got 0x%02X", sms.Reference)
	}
	if sms.PartNumber != 1 {
		t.Errorf("expected PartNumber 1, got %d", sms.PartNumber)
	}
	if sms.TotalParts != 2 {
		t.Errorf("expected TotalParts 2, got %d", sms.TotalParts)
	}
	if sms.Text != "Тест" {
		t.Errorf("expected stripped payload 'Тест', got %q", sms.Text)
	}
}
