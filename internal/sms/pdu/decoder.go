package pdu

import (
	"encoding/hex"
	"fmt"
)

// DecodeSMS decodes a hex-encoded SMS-DELIVER PDU string into DecodedSMS.
func DecodeSMS(pduHex string) (*DecodedSMS, error) {
	data, err := hex.DecodeString(pduHex)
	if err != nil {
		return nil, fmt.Errorf("decoding hex: %w: %v", ErrInvalidHex, err)
	}
	if len(data) < 11 {
		return nil, fmt.Errorf("PDU too short (%d bytes): %w", len(data), ErrTruncatedPDU)
	}

	offset := 0

	// 1. SCA
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU at SCA: %w", ErrTruncatedPDU)
	}
	scaLen := int(data[offset])
	offset += 1 + scaLen
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU after SCA: %w", ErrTruncatedPDU)
	}

	// 2. First Octet
	firstOctet := data[offset]
	hasUDH := (firstOctet & 0x40) != 0
	offset++

	// 3. OA
	if offset+2 > len(data) {
		return nil, fmt.Errorf("truncated PDU in OA header: %w", ErrTruncatedPDU)
	}
	digitCount := int(data[offset])
	offset++
	toa := data[offset]
	offset++
	bcdLen := (digitCount + 1) / 2
	if offset+bcdLen > len(data) {
		return nil, fmt.Errorf("truncated PDU in OA digits: %w", ErrTruncatedPDU)
	}
	from := DecodeAddress(data[offset:offset+bcdLen], digitCount, toa)
	offset += bcdLen

	// 4. PID & DCS
	if offset+2 > len(data) {
		return nil, fmt.Errorf("truncated PDU in PID/DCS: %w", ErrTruncatedPDU)
	}
	_ = data[offset] // PID
	offset++
	dcs := data[offset]
	offset++

	encoding := EncodingGSM7
	if (dcs & 0x0C) == 0x08 {
		encoding = EncodingUCS2
	}

	// 5. SCTS
	if offset+7 > len(data) {
		return nil, fmt.Errorf("truncated PDU in SCTS: %w", ErrTruncatedPDU)
	}
	timestamp := DecodeTimestamp(data[offset : offset+7])
	offset += 7

	// 6. UDL
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated PDU in UDL: %w", ErrTruncatedPDU)
	}
	udl := int(data[offset])
	offset++

	res := &DecodedSMS{
		From:      from,
		Timestamp: timestamp,
		Encoding:  encoding,
		HasUDH:    hasUDH,
	}

	// 7. UD & UDH
	if hasUDH {
		parseUDH(data, &offset, udl, encoding, res)
	} else {
		parsePlainUD(data[offset:], udl, encoding, res)
	}

	return res, nil
}

func parseUDH(data []byte, offset *int, udl int, enc Encoding, res *DecodedSMS) {
	if *offset >= len(data) {
		return
	}
	udhLen := int(data[*offset])
	udhTotalBytes := 1 + udhLen
	if *offset+udhTotalBytes > len(data) {
		return
	}
	udhData := data[*offset : *offset+udhTotalBytes]
	*offset += udhTotalBytes

	// Parse 8-bit or 16-bit concatenated SMS IE
	if len(udhData) >= 6 && udhData[1] == 0x00 { // 8-bit ref
		res.Reference = udhData[3]
		res.TotalParts = int(udhData[4])
		res.PartNumber = int(udhData[5])
	} else if len(udhData) >= 7 && udhData[1] == 0x08 { // 16-bit ref
		res.Reference = udhData[4]
		res.TotalParts = int(udhData[5])
		res.PartNumber = int(udhData[6])
	}

	if enc == EncodingUCS2 {
		if *offset <= len(data) {
			res.Text = DecodeUCS2(data[*offset:])
		}
	} else {
		udhBits := udhTotalBytes * 8
		padBits := (7 - (udhBits % 7)) % 7
		septetsToSkip := (udhBits + padBits) / 7
		remainingSeptets := udl - septetsToSkip
		if *offset <= len(data) && remainingSeptets > 0 {
			septets := UnpackSeptets(data[*offset:], remainingSeptets, padBits)
			res.Text = DecodeGSM7(septets)
		}
	}
}

func parsePlainUD(udBytes []byte, udl int, enc Encoding, res *DecodedSMS) {
	if len(udBytes) == 0 {
		return
	}
	if enc == EncodingUCS2 {
		res.Text = DecodeUCS2(udBytes)
	} else {
		septets := UnpackSeptets(udBytes, udl, 0)
		res.Text = DecodeGSM7(septets)
	}
}
