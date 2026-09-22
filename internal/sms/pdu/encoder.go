package pdu

// EncodeSMS encodes a recipient and text message into one or more PDUs.
// If requestDeliveryReport is true, TP-SRR bit (0x20) is set in the first octet.
// Unimplemented stub for TDD.
func EncodeSMS(recipient, text string, enc Encoding, requestDeliveryReport bool) ([]PDU, error) {
	// STUB for TDD: will fail tests
	return nil, nil
}
