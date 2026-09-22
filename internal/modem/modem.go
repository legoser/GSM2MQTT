// Package modem defines the interfaces for GSM modem drivers.
package modem

import "context"

// Driver is the interface that all modem drivers must implement.
// Each driver encapsulates the AT command specifics of a particular modem type.
type Driver interface {
	// Init initializes the modem and prepares it for operation.
	Init(ctx context.Context) error

	// Identify returns information about the connected modem.
	Identify() (*Info, error)

	// Close releases modem resources and closes the connection.
	Close() error

	SMSSender
	SMSReader
	Caller
	StatusProvider
	USSDSender
	RawCommander
}

// SMSSender sends SMS messages.
type SMSSender interface {
	SendSMS(number, text string) (messageRef byte, err error)
}

// SMSReader reads SMS messages from the modem.
type SMSReader interface {
	ListSMS(filter SMSFilter) ([]SMS, error)
	DeleteSMS(index int) error
}

// Caller handles voice call operations.
type Caller interface {
	Dial(number string) error
	Answer() error
	Hangup() error
	SendDTMF(digit string) error
}

// StatusProvider retrieves modem status information.
type StatusProvider interface {
	SignalQuality() (int, error)
	NetworkRegistration() (*NetworkStatus, error)
	OperatorName() (string, error)
	SIMStatus() (SIMState, error)
}

// USSDSender sends USSD requests.
type USSDSender interface {
	SendUSSD(code string) (string, error)
}

// RawCommander sends raw AT commands.
type RawCommander interface {
	SendRawAT(cmd string) (string, error)
}

// Info contains identification information about a modem.
type Info struct {
	Manufacturer string
	Model        string
	Revision     string
	IMEI         string
	IMSI         string
}

// SMS represents a received SMS message.
type SMS struct {
	Index     int
	From      string
	Text      string
	Timestamp string
	Encoding  string
}

// SMSFilter defines criteria for listing SMS messages.
type SMSFilter string

const (
	SMSFilterAll    SMSFilter = "ALL"
	SMSFilterUnread SMSFilter = "REC UNREAD"
	SMSFilterRead   SMSFilter = "REC READ"
)

// NetworkStatus represents the modem's network registration status.
type NetworkStatus struct {
	Registered bool
	Roaming    bool
	Technology string // GSM, GPRS, EDGE, 3G, LTE
}

// SIMState represents the state of the SIM card.
type SIMState string

const (
	SIMReady       SIMState = "READY"
	SIMPINRequired SIMState = "SIM PIN"
	SIMPUKRequired SIMState = "SIM PUK"
	SIMAbsent      SIMState = "NO SIM"
	SIMError       SIMState = "ERROR"
)
