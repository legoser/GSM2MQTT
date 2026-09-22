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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.actual)
			}
		})
	}
}
