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

// Alert returns the urgent alert notification topic.
func (t *Topics) Alert() string {
	return fmt.Sprintf("%s/modem/%s/alert", t.prefix, t.modemID)
}

// Event returns the topic for publishing structured lifecycle and warning events.
func (t *Topics) Event() string {
	return fmt.Sprintf("%s/modem/%s/event", t.prefix, t.modemID)
}

// Diagnostic returns the topic for detailed failure diagnostics.
func (t *Topics) Diagnostic() string {
	return fmt.Sprintf("%s/modem/%s/diagnostic", t.prefix, t.modemID)
}

// Balance returns the topic for publishing current account balance.
func (t *Topics) Balance() string {
	return fmt.Sprintf("%s/modem/%s/balance", t.prefix, t.modemID)
}

// TariffSetPreset returns the topic for dynamically setting the operator preset.
func (t *Topics) TariffSetPreset() string {
	return fmt.Sprintf("%s/modem/%s/tariff/set_preset", t.prefix, t.modemID)
}

// TariffSet returns the topic for dynamically updating tariff configuration.
func (t *Topics) TariffSet() string {
	return fmt.Sprintf("%s/modem/%s/tariff/set", t.prefix, t.modemID)
}

// TariffReset returns the topic for resetting monthly quota usage.
func (t *Topics) TariffReset() string {
	return fmt.Sprintf("%s/modem/%s/tariff/reset", t.prefix, t.modemID)
}

// AccountingStatus returns the topic for reporting SMS and data quotas.
func (t *Topics) AccountingStatus() string {
	return fmt.Sprintf("%s/modem/%s/accounting/status", t.prefix, t.modemID)
}

// AccountingAlert returns the topic for quota exhaustion warnings.
func (t *Topics) AccountingAlert() string {
	return fmt.Sprintf("%s/modem/%s/accounting/alert", t.prefix, t.modemID)
}

// LWT returns the service-level Last Will and Testament topic.
func (t *Topics) LWT() string {
	return fmt.Sprintf("%s/status", t.prefix)
}

// PoolStatus returns the cluster pool status topic.
func (t *Topics) PoolStatus() string {
	return fmt.Sprintf("%s/pool/status", t.prefix)
}

// PoolSMSSend returns the common pool SMS send topic.
func (t *Topics) PoolSMSSend() string {
	return fmt.Sprintf("%s/sms/send", t.prefix)
}

// PoolCallDial returns the common pool call dial topic.
func (t *Topics) PoolCallDial() string {
	return fmt.Sprintf("%s/call/dial", t.prefix)
}

// PoolCallHangup returns the common pool call hangup topic.
func (t *Topics) PoolCallHangup() string {
	return fmt.Sprintf("%s/call/hangup", t.prefix)
}

// PoolUSSDSend returns the common pool USSD send topic.
func (t *Topics) PoolUSSDSend() string {
	return fmt.Sprintf("%s/ussd/send", t.prefix)
}
