// Package mqtt provides MQTT protocol client, topic hierarchy, and Home Assistant discovery.
package mqtt

import "fmt"

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
	return fmt.Sprintf("%s/modem/%s/sms/send", t.prefix, t.modemID)
}

// SMSStatus returns the topic for publishing delivery reports.
func (t *Topics) SMSStatus() string {
	return fmt.Sprintf("%s/modem/%s/sms/status", t.prefix, t.modemID)
}

// SMSReceived returns the topic for publishing received SMS messages.
func (t *Topics) SMSReceived() string {
	return fmt.Sprintf("%s/modem/%s/sms/received", t.prefix, t.modemID)
}

// CallDial returns the topic for requesting an outgoing call.
func (t *Topics) CallDial() string {
	return fmt.Sprintf("%s/modem/%s/call/dial", t.prefix, t.modemID)
}

// CallHangup returns the topic for terminating an active call.
func (t *Topics) CallHangup() string {
	return fmt.Sprintf("%s/modem/%s/call/hangup", t.prefix, t.modemID)
}

// CallIncoming returns the topic for announcing incoming calls.
func (t *Topics) CallIncoming() string {
	return fmt.Sprintf("%s/modem/%s/call/incoming", t.prefix, t.modemID)
}

// CallDTMF returns the topic for reporting received DTMF tones.
func (t *Topics) CallDTMF() string {
	return fmt.Sprintf("%s/modem/%s/call/dtmf", t.prefix, t.modemID)
}

// SignalStrength returns the topic for publishing signal quality.
func (t *Topics) SignalStrength() string {
	return fmt.Sprintf("%s/modem/%s/signal", t.prefix, t.modemID)
}

// USSDSend returns the topic for requesting USSD execution.
func (t *Topics) USSDSend() string {
	return fmt.Sprintf("%s/modem/%s/ussd/send", t.prefix, t.modemID)
}

// USSDResponse returns the topic for publishing USSD replies.
func (t *Topics) USSDResponse() string {
	return fmt.Sprintf("%s/modem/%s/ussd/response", t.prefix, t.modemID)
}

// CommandRaw returns the topic for sending raw AT commands.
func (t *Topics) CommandRaw() string {
	return fmt.Sprintf("%s/modem/%s/command/raw", t.prefix, t.modemID)
}

// CommandResponse returns the topic for receiving raw AT responses.
func (t *Topics) CommandResponse() string {
	return fmt.Sprintf("%s/modem/%s/command/response", t.prefix, t.modemID)
}

// Health returns the topic for modem health updates.
func (t *Topics) Health() string {
	return fmt.Sprintf("%s/modem/%s/health", t.prefix, t.modemID)
}

// LWT returns the service-level Last Will and Testament topic.
func (t *Topics) LWT() string {
	return fmt.Sprintf("%s/status", t.prefix)
}
