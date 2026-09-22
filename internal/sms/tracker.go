package sms

import (
	"time"

	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

// DeliveryStatus represents the delivery outcome of an SMS.
type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusExpired   DeliveryStatus = "expired"
)

// DeliveryEvent represents a status update to be published to MQTT.
type DeliveryEvent struct {
	MessageRef byte           `json:"ref"`
	To         string         `json:"to"`
	Status     DeliveryStatus `json:"status"`
	ModemID    string         `json:"modem_id"`
}

// Tracker tracks SMS delivery status and generates timeout events.
type Tracker struct {
	timeout  time.Duration
	onUpdate func(event DeliveryEvent)
}

// NewTracker creates a new delivery report Tracker.
func NewTracker(timeout time.Duration, onUpdate func(event DeliveryEvent)) *Tracker {
	return &Tracker{
		timeout:  timeout,
		onUpdate: onUpdate,
	}
}

// Track registers a sent SMS for delivery tracking.
func (t *Tracker) Track(ref byte, to, text, modemID string) {
	// STUB for TDD: will fail tests
}

// HandleReport processes an incoming delivery report.
func (t *Tracker) HandleReport(report *pdu.StatusReport) {
	// STUB for TDD: will fail tests
}
