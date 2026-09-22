// Package mqtt provides MQTT protocol client, topic hierarchy, and Home Assistant discovery.
package mqtt

// Topics manages MQTT topic generation for a given topic prefix and modem ID.
type Topics struct {
	prefix  string
	modemID string
}

// NewTopics creates a Topics helper.
func NewTopics(prefix, modemID string) *Topics {
	return &Topics{
		prefix:  prefix,
		modemID: modemID,
	}
}

// SMSSend returns the topic for requesting outgoing SMS.
func (t *Topics) SMSSend() string {
	// STUB for TDD: will fail tests
	return ""
}

// SMSStatus returns the topic for publishing delivery reports.
func (t *Topics) SMSStatus() string {
	// STUB for TDD: will fail tests
	return ""
}

// SMSReceived returns the topic for publishing received SMS messages.
func (t *Topics) SMSReceived() string {
	// STUB for TDD: will fail tests
	return ""
}

// CallDial returns the topic for requesting an outgoing call.
func (t *Topics) CallDial() string {
	// STUB for TDD: will fail tests
	return ""
}

// CallIncoming returns the topic for announcing incoming calls.
func (t *Topics) CallIncoming() string {
	// STUB for TDD: will fail tests
	return ""
}

// CallDTMF returns the topic for reporting received DTMF tones.
func (t *Topics) CallDTMF() string {
	// STUB for TDD: will fail tests
	return ""
}

// SignalStrength returns the topic for publishing signal quality.
func (t *Topics) SignalStrength() string {
	// STUB for TDD: will fail tests
	return ""
}

// LWT returns the service-level Last Will and Testament topic.
func (t *Topics) LWT() string {
	// STUB for TDD: will fail tests
	return ""
}
