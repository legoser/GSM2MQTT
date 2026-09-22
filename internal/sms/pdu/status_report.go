package pdu

import (
	"encoding/hex"
	"fmt"
)

// DecodeStatusReport decodes an incoming SMS-STATUS-REPORT PDU (+CDS).
func DecodeStatusReport(pduHex string) (*StatusReport, error) {
	data, err := hex.DecodeString(pduHex)
	if err != nil {
		return nil, fmt.Errorf("decoding hex: %w", err)
	}
	if len(data) < 18 {
		return nil, fmt.Errorf("status report PDU too short (%d bytes)", len(data))
	}

	offset := 0

	// 1. SCA
	scaLen := int(data[offset])
	offset += 1 + scaLen
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU after SCA")
	}

	// 2. First Octet
	_ = data[offset]
	offset++

	// 3. TP-MR
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-MR")
	}
	msgRef := data[offset]
	offset++

	// 4. TP-RA
	if offset+2 > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-RA header")
	}
	digitCount := int(data[offset])
	offset++
	toa := data[offset]
	offset++
	bcdLen := (digitCount + 1) / 2
	if offset+bcdLen > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-RA digits")
	}
	recipient := DecodeAddress(data[offset:offset+bcdLen], digitCount, toa)
	offset += bcdLen

	// 5. TP-SCTS
	if offset+7 > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-SCTS")
	}
	scTimestamp := DecodeTimestamp(data[offset : offset+7])
	offset += 7

	// 6. TP-DT
	if offset+7 > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-DT")
	}
	discharge := DecodeTimestamp(data[offset : offset+7])
	offset += 7

	// 7. TP-ST
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-ST")
	}
	statusCode := data[offset]

	return &StatusReport{
		MessageRef:  msgRef,
		Recipient:   recipient,
		SCTimestamp: scTimestamp,
		Discharge:   discharge,
		StatusCode:  statusCode,
		Delivered:   statusCode <= 0x1F,
		Temporary:   statusCode >= 0x20 && statusCode <= 0x3F,
		Permanent:   statusCode >= 0x40 && statusCode <= 0x5F,
	}, nil
}
