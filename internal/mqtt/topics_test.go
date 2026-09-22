package mqtt

import "testing"

func TestTopics_Generation(t *testing.T) {
	topics := NewTopics("gsm2mqtt", "siemens_tc35")

	tests := []struct {
		name     string
		actual   string
		expected string
	}{
		{
			name:     "LWT topic",
			actual:   topics.LWT(),
			expected: "gsm2mqtt/status",
		},
		{
			name:     "SMS Send topic",
			actual:   topics.SMSSend(),
			expected: "gsm2mqtt/modem/siemens_tc35/sms/send",
		},
		{
			name:     "SMS Status (delivery reports) topic",
			actual:   topics.SMSStatus(),
			expected: "gsm2mqtt/modem/siemens_tc35/sms/status",
		},
		{
			name:     "SMS Received topic",
			actual:   topics.SMSReceived(),
			expected: "gsm2mqtt/modem/siemens_tc35/sms/received",
		},
		{
			name:     "Call Dial topic",
			actual:   topics.CallDial(),
			expected: "gsm2mqtt/modem/siemens_tc35/call/dial",
		},
		{
			name:     "Call Hangup topic",
			actual:   topics.CallHangup(),
			expected: "gsm2mqtt/modem/siemens_tc35/call/hangup",
		},
		{
			name:     "Call Incoming topic",
			actual:   topics.CallIncoming(),
			expected: "gsm2mqtt/modem/siemens_tc35/call/incoming",
		},
		{
			name:     "Call DTMF topic",
			actual:   topics.CallDTMF(),
			expected: "gsm2mqtt/modem/siemens_tc35/call/dtmf",
		},
		{
			name:     "Signal Strength topic",
			actual:   topics.SignalStrength(),
			expected: "gsm2mqtt/modem/siemens_tc35/signal",
		},
		{
			name:     "USSD Send topic",
			actual:   topics.USSDSend(),
			expected: "gsm2mqtt/modem/siemens_tc35/ussd/send",
		},
		{
			name:     "USSD Response topic",
			actual:   topics.USSDResponse(),
			expected: "gsm2mqtt/modem/siemens_tc35/ussd/response",
		},
		{
			name:     "Command Raw topic",
			actual:   topics.CommandRaw(),
			expected: "gsm2mqtt/modem/siemens_tc35/command/raw",
		},
		{
			name:     "Command Response topic",
			actual:   topics.CommandResponse(),
			expected: "gsm2mqtt/modem/siemens_tc35/command/response",
		},
		{
			name:     "Health topic",
			actual:   topics.Health(),
			expected: "gsm2mqtt/modem/siemens_tc35/health",
		},
		{
			name:     "Alert topic",
			actual:   topics.Alert(),
			expected: "gsm2mqtt/modem/siemens_tc35/alert",
		},
		{
			name:     "Diagnostic topic",
			actual:   topics.Diagnostic(),
			expected: "gsm2mqtt/modem/siemens_tc35/diagnostic",
		},
		{
			name:     "Balance topic",
			actual:   topics.Balance(),
			expected: "gsm2mqtt/modem/siemens_tc35/balance",
		},
		{
			name:     "TariffSetPreset topic",
			actual:   topics.TariffSetPreset(),
			expected: "gsm2mqtt/modem/siemens_tc35/tariff/set_preset",
		},
		{
			name:     "AccountingStatus topic",
			actual:   topics.AccountingStatus(),
			expected: "gsm2mqtt/modem/siemens_tc35/accounting/status",
		},
		{
			name:     "AccountingAlert topic",
			actual:   topics.AccountingAlert(),
			expected: "gsm2mqtt/modem/siemens_tc35/accounting/alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.actual)
			}
		})
	}
}
