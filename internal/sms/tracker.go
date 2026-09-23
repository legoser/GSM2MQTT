package sms

import (
	"log/slog"
	"sync"
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

type trackedSMS struct {
	ref     byte
	to      string
	text    string
	modemID string
	timer   *time.Timer
}

// Tracker tracks SMS delivery status and generates timeout events.
type Tracker struct {
	mu       sync.Mutex
	timeout  time.Duration
	onUpdate func(event DeliveryEvent)
	pending  map[byte]*trackedSMS
}

// NewTracker creates a new delivery report Tracker.
func NewTracker(timeout time.Duration, onUpdate func(event DeliveryEvent)) *Tracker {
	return &Tracker{
		timeout:  timeout,
		onUpdate: onUpdate,
		pending:  make(map[byte]*trackedSMS),
	}
}

// Track registers a sent SMS for delivery tracking.
func (t *Tracker) Track(ref byte, to, text, modemID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	item := &trackedSMS{
		ref:     ref,
		to:      to,
		text:    text,
		modemID: modemID,
	}

	if t.timeout > 0 {
		item.timer = time.AfterFunc(t.timeout, func() {
			t.handleTimeout(ref)
		})
	}

	t.pending[ref] = item
	slog.Debug("tracking SMS delivery", slog.String("modem", modemID), slog.String("to", to), slog.Int("ref", int(ref)))

	if t.onUpdate != nil {
		t.onUpdate(DeliveryEvent{
			MessageRef: ref,
			To:         to,
			Status:     DeliveryStatusPending,
			ModemID:    modemID,
		})
	}
}

// HandleReport processes an incoming delivery report.
func (t *Tracker) HandleReport(report *pdu.StatusReport) {
	t.mu.Lock()
	defer t.mu.Unlock()

	item, ok := t.pending[report.MessageRef]
	if !ok {
		return
	}

	if item.timer != nil {
		item.timer.Stop()
	}
	delete(t.pending, report.MessageRef)

	status := DeliveryStatusDelivered
	if !report.Delivered {
		if report.Permanent {
			status = DeliveryStatusFailed
		} else {
			status = DeliveryStatusPending
		}
	}

	slog.Info("SMS delivery report processed", slog.String("modem", item.modemID), slog.String("to", item.to), slog.Int("ref", int(report.MessageRef)), slog.String("status", string(status)))

	if t.onUpdate != nil {
		t.onUpdate(DeliveryEvent{
			MessageRef: report.MessageRef,
			To:         item.to,
			Status:     status,
			ModemID:    item.modemID,
		})
	}
}

func (t *Tracker) handleTimeout(ref byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	item, ok := t.pending[ref]
	if !ok {
		return
	}

	delete(t.pending, ref)
	slog.Warn("SMS delivery report expired (timeout)", slog.String("modem", item.modemID), slog.String("to", item.to), slog.Int("ref", int(ref)))

	if t.onUpdate != nil {
		t.onUpdate(DeliveryEvent{
			MessageRef: ref,
			To:         item.to,
			Status:     DeliveryStatusExpired,
			ModemID:    item.modemID,
		})
	}
}
