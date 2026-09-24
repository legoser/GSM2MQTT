package sms

import (
	"fmt"
	"log/slog"
	"strings"
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
	modemID string
	timer   *time.Timer
}

// Tracker tracks SMS delivery status and generates timeout events.
type Tracker struct {
	mu       sync.Mutex
	timeout  time.Duration
	onUpdate func(event DeliveryEvent)
	pending  map[string]*trackedSMS
}

// NewTracker creates a new delivery report Tracker.
func NewTracker(timeout time.Duration, onUpdate func(event DeliveryEvent)) *Tracker {
	return &Tracker{
		timeout:  timeout,
		onUpdate: onUpdate,
		pending:  make(map[string]*trackedSMS),
	}
}

func makeKey(to string, ref byte) string {
	return fmt.Sprintf("%s:%d", canonicalRecipient(to), ref)
}

// canonicalRecipient best-effort normalizes a recipient for key matching:
// strips formatting, converts national 8-prefix to +7. Status reports may
// arrive with national TOA (no "+") while Track stores E.164 — without this
// the report would miss and the entry would leak until timeout.
func canonicalRecipient(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	hasPlus := strings.HasPrefix(trimmed, "+")
	var digits strings.Builder
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if d == "" {
		return trimmed
	}
	if hasPlus {
		return "+" + d
	}
	if len(d) == 11 && d[0] == '8' {
		return "+7" + d[1:]
	}
	return d
}

// Track registers a sent SMS for delivery tracking.
func (t *Tracker) Track(ref byte, to string, modemID string) {
	t.mu.Lock()

	key := makeKey(to, ref)

	// If overwriting, stop the old timer
	if old, ok := t.pending[key]; ok && old.timer != nil {
		old.timer.Stop()
	}

	item := &trackedSMS{
		ref:     ref,
		to:      to,
		modemID: modemID,
	}

	if t.timeout > 0 {
		item.timer = time.AfterFunc(t.timeout, func() {
			t.handleTimeout(key)
		})
	}

	t.pending[key] = item
	t.mu.Unlock()
	slog.Debug("tracking SMS delivery", slog.String("modem", modemID), slog.String("to", to), slog.Int("ref", int(ref)))

	// Callbacks run outside the lock (no goroutine: bounded, ordered).
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
	key := makeKey(report.Recipient, report.MessageRef)
	item, ok := t.pending[key]
	if !ok {
		t.mu.Unlock()
		return
	}

	if item.timer != nil {
		item.timer.Stop()
	}
	delete(t.pending, key)
	t.mu.Unlock()

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

func (t *Tracker) handleTimeout(key string) {
	t.mu.Lock()
	item, ok := t.pending[key]
	if !ok {
		t.mu.Unlock()
		return
	}

	delete(t.pending, key)
	t.mu.Unlock()
	slog.Warn("SMS delivery report expired (timeout)", slog.String("modem", item.modemID), slog.String("to", item.to), slog.Int("ref", int(item.ref)))

	if t.onUpdate != nil {
		t.onUpdate(DeliveryEvent{
			MessageRef: item.ref,
			To:         item.to,
			Status:     DeliveryStatusExpired,
			ModemID:    item.modemID,
		})
	}
}
