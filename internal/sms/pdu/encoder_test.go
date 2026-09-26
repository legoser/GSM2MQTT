package pdu

import (
	"fmt"
	"strings"
	"testing"
)

func TestEncodeSMS_SingleLatin(t *testing.T) {
	recipient := "+79123456789"
	text := "Hello World!"

	pdus, err := EncodeSMS(recipient, text, EncodingAuto, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdus) != 1 {
		t.Fatalf("expected 1 PDU, got %d", len(pdus))
	}

	pdu := pdus[0]
	if pdu.HasUDH {
		t.Errorf("expected HasUDH to be false for single SMS")
	}
	if pdu.Encoding != EncodingGSM7 {
		t.Errorf("expected EncodingGSM7, got %v", pdu.Encoding)
	}
	if pdu.CommandLength <= 0 {
		t.Errorf("expected positive CommandLength, got %d", pdu.CommandLength)
	}
	if !strings.HasPrefix(pdu.Hex, "00") { // 00 indicates default SMSC
		t.Errorf("expected PDU to start with 00 (default SMSC), got %q", pdu.Hex)
	}
}

func TestEncodeSMS_SingleCyrillic(t *testing.T) {
	recipient := "+79123456789"
	text := "Привет"

	pdus, err := EncodeSMS(recipient, text, EncodingAuto, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdus) != 1 {
		t.Fatalf("expected 1 PDU, got %d", len(pdus))
	}

	pdu := pdus[0]
	if pdu.Encoding != EncodingUCS2 {
		t.Errorf("expected EncodingUCS2 for Cyrillic, got %v", pdu.Encoding)
	}
	// "Привет" in UCS-2 BE hex is 041F 0440 0438 0432 0435 0442
	expectedHexSubstr := "041F04400438043204350442"
	if !strings.Contains(strings.ToUpper(pdu.Hex), expectedHexSubstr) {
		t.Errorf("expected PDU hex to contain UCS-2 %q, got %q", expectedHexSubstr, pdu.Hex)
	}
}

func TestEncodeSMS_DeliveryReportRequested(t *testing.T) {
	recipient := "+79123456789"
	text := "Alarm test"

	pdus, err := EncodeSMS(recipient, text, EncodingAuto, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdus) == 0 {
		t.Fatal("expected at least 1 PDU")
	}

	// In SMS-SUBMIT TPDU with default SMSC "00", the first octet is at index 2..4 in Hex string.
	// First octet with bitSRR (0x20) and MTI submit (0x01) and validity (0x10) is 0x31 ("31").
	// Without SRR it would be 0x11 ("11").
	if len(pdus[0].Hex) < 4 {
		t.Fatalf("PDU hex too short: %q", pdus[0].Hex)
	}
	firstOctetHex := pdus[0].Hex[2:4]
	if firstOctetHex != "31" && firstOctetHex != "35" {
		t.Errorf("expected first octet to have SRR bit set (0x31 or 0x35), got %q", firstOctetHex)
	}
}

func TestEncodeSMS_MultipartCyrillic(t *testing.T) {
	recipient := "+79123456789"
	// 80 characters of Cyrillic > 70 character limit for single UCS-2 SMS
	text := "Внимание! Сработал датчик движения в зоне 3. Пожалуйста проверьте периметр дома."

	pdus, err := EncodeSMS(recipient, text, EncodingAuto, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdus) < 2 {
		t.Fatalf("expected at least 2 PDUs for %d runes of Cyrillic, got %d", len([]rune(text)), len(pdus))
	}

	for i, pdu := range pdus {
		if !pdu.HasUDH {
			t.Errorf("pdu[%d] expected HasUDH = true for multipart", i)
		}
		if pdu.PartNumber != i+1 {
			t.Errorf("pdu[%d] expected PartNumber %d, got %d", i, i+1, pdu.PartNumber)
		}
		if pdu.TotalParts != len(pdus) {
			t.Errorf("pdu[%d] expected TotalParts %d, got %d", i, len(pdus), pdu.TotalParts)
		}
	}
}

func TestEncodeSMS_MultipartEmoji(t *testing.T) {
	recipient := "+79964126670"
	text := "🚨 *ПОЖАРНАЯ ТРЕВОГА!*\n⚠️ Сработал датчик дыма в гостиной ⏰ *Время:* 15:45:00"

	pdus, err := EncodeSMS(recipient, text, EncodingAuto, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdus) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(pdus))
	}
	for i, p := range pdus {
		// CommandLength in PDU mode is (len(TPDU) / 2)
		// For UCS-2 with UDH, TPDU is: 1 (firstOctet) + 1 (MR) + DA + 1 (PID) + 1 (DCS) + 1 (VP) + 1 (UDL) + UDL bytes.
		// Maximum UDL must never exceed 140 (0x8C) bytes!
		t.Logf("Part %d: CmdLen=%d, HexLen=%d, Hex=%s", i+1, p.CommandLength, len(p.Hex), p.Hex)
		// Default SMSC is "00" (1 byte), followed by TPDU.
		// DA for "+79964126670": 0B 91 97 69 14 62 76 F0 (8 bytes)
		// Header up to UDL: 00 (SMSC) + 51 (1) + 00 (1) + DA (8) + 00 (1) + 08 (1) + AA (1) = 14 bytes = 28 hex chars
		// Next byte (chars 28..30) is UDL in hex!
		udlHex := p.Hex[28:30]
		var udl int
		_, _ = fmt.Sscanf(udlHex, "%02X", &udl)
		if udl > 140 {
			t.Errorf("part %d UDL %d (0x%s) exceeds 140-byte GSM limit!", i+1, udl, udlHex)
		}
	}
}
