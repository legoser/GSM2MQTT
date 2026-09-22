package pdu

import (
	"encoding/hex"
	"fmt"
)

// DecodeStatusReport decodes an incoming SMS-STATUS-REPORT PDU (+CDS).
func DecodeStatusReport(pduHex string) (*StatusReport, error) {
	data, err := hex.DecodeString(pduHex)
	if err != nil {
		return nil, fmt.Errorf("decoding hex: %w: %v", ErrInvalidHex, err)
	}
	if len(data) < 18 {
		return nil, fmt.Errorf("status report PDU too short (%d bytes): %w", len(data), ErrTruncatedPDU)
	}

	offset := 0

	// 1. SCA
	scaLen := int(data[offset])
	offset += 1 + scaLen
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU after SCA: %w", ErrTruncatedPDU)
	}

	// 2. First Octet (check MTI == 0x02 for SMS-STATUS-REPORT)
	firstOctet := data[offset]
	if (firstOctet & 0x03) != 0x02 {
		return nil, fmt.Errorf("%w: first octet 0x%02X", ErrNotStatusReport, firstOctet)
	}
	offset++

	// 3. TP-MR
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-MR: %w", ErrTruncatedPDU)
	}
	msgRef := data[offset]
	offset++

	// 4. TP-RA
	if offset+2 > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-RA header: %w", ErrTruncatedPDU)
	}
	digitCount := int(data[offset])
	offset++
	toa := data[offset]
	offset++
	bcdLen := (digitCount + 1) / 2
	if offset+bcdLen > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-RA digits: %w", ErrTruncatedPDU)
	}
	recipient := DecodeAddress(data[offset:offset+bcdLen], digitCount, toa)
	offset += bcdLen

	// 5. TP-SCTS
	if offset+7 > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-SCTS: %w", ErrTruncatedPDU)
	}
	scTimestamp := DecodeTimestamp(data[offset : offset+7])
	offset += 7

	// 6. TP-DT
	if offset+7 > len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-DT: %w", ErrTruncatedPDU)
	}
	discharge := DecodeTimestamp(data[offset : offset+7])
	offset += 7

	// 7. TP-ST
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU in TP-ST: %w", ErrTruncatedPDU)
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
