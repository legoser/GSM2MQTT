package pdu

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

var refCounter byte = 1

func nextRefNumber() byte {
	refCounter++
	if refCounter == 0 {
		refCounter = 1
	}
	return refCounter
}

// EncodeSMS encodes a recipient and text message into one or more PDUs.
func EncodeSMS(recipient, text string, enc Encoding, requestDeliveryReport bool) ([]PDU, error) {
	if strings.TrimSpace(recipient) == "" {
		return nil, fmt.Errorf("encoding SMS: %w", ErrEmptyRecipient)
	}
	if text == "" {
		return nil, fmt.Errorf("encoding SMS: %w", ErrEmptyText)
	}

	// 1. Determine encoding
	selectedEnc := enc
	if selectedEnc == EncodingAuto || selectedEnc == "" {
		if IsGSM7(text) {
			selectedEnc = EncodingGSM7
		} else {
			selectedEnc = EncodingUCS2
		}
	}

	runes := []rune(text)

	// 2. Determine limits
	var maxSingle, maxMultipart int
	if selectedEnc == EncodingGSM7 {
		maxSingle = 160
		maxMultipart = 153
	} else {
		maxSingle = 70
		maxMultipart = 67
	}

	// Calculate units (GSM7 septets or UCS2 uint16 code units, accounting for surrogate pairs)
	totalUnits := 0
	if selectedEnc == EncodingUCS2 {
		for _, r := range runes {
			if r > 0xFFFF {
				totalUnits += 2 // surrogate pair consumes 2 UTF-16 code units (4 bytes)
			} else {
				totalUnits++
			}
		}
	} else {
		totalUnits = len(runes)
	}

	// 3. Single or multipart
	if totalUnits <= maxSingle {
		pdu, err := encodeSingle(recipient, runes, selectedEnc, requestDeliveryReport)
		if err != nil {
			return nil, err
		}
		return []PDU{pdu}, nil
	}

	// Multipart
	ref := nextRefNumber()
	chunks := splitChunks(runes, selectedEnc, maxMultipart)

	pdus := make([]PDU, len(chunks))
	for i, chunk := range chunks {
		p, err := encodePart(recipient, chunk, selectedEnc, requestDeliveryReport, ref, i+1, len(chunks))
		if err != nil {
			return nil, err
		}
		pdus[i] = p
	}

	return pdus, nil
}

func encodeSingle(recipient string, runes []rune, enc Encoding, srr bool) (PDU, error) {
	firstOctet := byte(0x11) // SMS-SUBMIT, relative validity
	if srr {
		firstOctet |= 0x20 // TP-SRR
	}

	var udl byte
	var udHex string
	var dcs byte

	if enc == EncodingGSM7 {
		dcs = 0x00
		septets := EncodeGSM7(string(runes))
		udl = byte(len(septets))
		packed := PackSeptets(septets, 0)
		udHex = hexString(packed)
	} else {
		dcs = 0x08
		ucs2Bytes := EncodeUCS2(string(runes))
		udl = byte(len(ucs2Bytes))
		udHex = hexString(ucs2Bytes)
	}

	return buildPDU(recipient, firstOctet, dcs, udl, udHex, false, enc, 1, 1)
}

func encodePart(recipient string, runes []rune, enc Encoding, srr bool, ref byte, partNum, totalParts int) (PDU, error) {
	firstOctet := byte(0x11 | 0x40) // SMS-SUBMIT, relative validity, UDHI
	if srr {
		firstOctet |= 0x20 // TP-SRR
	}

	udh := []byte{0x05, 0x00, 0x03, ref, byte(totalParts), byte(partNum)}

	var udl byte
	var udHex string
	var dcs byte

	if enc == EncodingGSM7 {
		dcs = 0x00
		septets := EncodeGSM7(string(runes))
		// 6 bytes UDH = 48 bits. + 1 padding bit = 49 bits = 7 septets.
		udl = byte(7 + len(septets))
		// Pack septets with startBit = 1 (after 6 UDH bytes + 1 pad bit)
		packedText := PackSeptets(septets, 1)
		udHex = hexString(udh) + hexString(packedText)
	} else {
		dcs = 0x08
		ucs2Bytes := EncodeUCS2(string(runes))
		udl = byte(len(udh) + len(ucs2Bytes))
		udHex = hexString(udh) + hexString(ucs2Bytes)
	}

	return buildPDU(recipient, firstOctet, dcs, udl, udHex, true, enc, partNum, totalParts)
}

func buildPDU(recipient string, firstOctet, dcs, udl byte, udHex string, hasUDH bool, enc Encoding, partNum, totalParts int) (PDU, error) {
	smscHex := "00" // Default SMSC stored in modem
	mrHex := "00"   // Modem generates TP-MR
	daHex := FormatAddressHex(recipient)
	pidHex := "00"
	dcsHex := fmt.Sprintf("%02X", dcs)
	vpHex := "AA" // 4 days validity
	udlHex := fmt.Sprintf("%02X", udl)

	tpduHex := fmt.Sprintf("%02X%s%s%s%s%s%s%s", firstOctet, mrHex, daHex, pidHex, dcsHex, vpHex, udlHex, udHex)
	fullHex := smscHex + tpduHex

	commandLength := len(tpduHex) / 2

	return PDU{
		CommandLength: commandLength,
		Hex:           fullHex,
		HasUDH:        hasUDH,
		Encoding:      enc,
		PartNumber:    partNum,
		TotalParts:    totalParts,
	}, nil
}

func hexString(b []byte) string {
	return strings.ToUpper(hex.EncodeToString(b))
}

func splitChunks(runes []rune, enc Encoding, maxUnits int) [][]rune {
	if enc != EncodingUCS2 {
		var chunks [][]rune
		for len(runes) > 0 {
			chunkSize := maxUnits
			if len(runes) < chunkSize {
				chunkSize = len(runes)
			}
			chunks = append(chunks, runes[:chunkSize])
			runes = runes[chunkSize:]
		}
		return chunks
	}

	var chunks [][]rune
	var current []rune
	currentUnits := 0
	for _, r := range runes {
		units := 1
		if r > 0xFFFF {
			units = 2 // surrogate pair consumes 2 UTF-16 code units (4 bytes)
		}
		if currentUnits+units > maxUnits {
			chunks = append(chunks, current)
			current = []rune{r}
			currentUnits = units
		} else {
			current = append(current, r)
			currentUnits += units
		}
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

func init() {
	refCounter = byte(time.Now().UnixNano() & 0xFF)
	if refCounter == 0 {
		refCounter = 1
	}
}
