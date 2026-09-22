package mqtt

// DiscoveryMessage represents a Home Assistant MQTT Auto Discovery payload.
type DiscoveryMessage struct {
	Topic   string
	Payload []byte
}

// BuildSignalDiscovery generates the MQTT Auto-Discovery configuration for GSM signal strength sensor.
// Unimplemented stub for TDD.
func BuildSignalDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	// STUB for TDD: will fail tests
	return nil, nil
}
