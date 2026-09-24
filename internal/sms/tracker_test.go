package sms

import (
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

func TestTracker_Delivered(t *testing.T) {
	var mu sync.Mutex
	var events []DeliveryEvent

	tracker := NewTracker(1*time.Second, func(ev DeliveryEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	tracker.Track(15, "+79991112233", "siemens_tc35")

	// Simulate delivery report arriving
	tracker.HandleReport(&pdu.StatusReport{
		MessageRef: 15,
		Recipient:  "+79991112233",
		Delivered:  true,
		StatusCode: 0x00,
	})

	mu.Lock()
	defer mu.Unlock()

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events (pending, delivered), got %d", len(events))
	}
	if events[0].Status != DeliveryStatusPending {
		t.Errorf("expected first event to be pending, got %v", events[0].Status)
	}
	if events[1].Status != DeliveryStatusDelivered {
		t.Errorf("expected second event to be delivered, got %v", events[1].Status)
	}
}

func TestTracker_TimeoutExpired(t *testing.T) {
	var mu sync.Mutex
	var events []DeliveryEvent

	// Short timeout for test
	tracker := NewTracker(50*time.Millisecond, func(ev DeliveryEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	tracker.Track(99, "+79991112233", "siemens_tc35")

	// Wait for timeout to fire
	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events (pending, expired), got %d", len(events))
	}
	lastEvent := events[len(events)-1]
	if lastEvent.Status != DeliveryStatusExpired {
		t.Errorf("expected last event to be expired, got %v", lastEvent.Status)
	}
}

func TestTracker_UnknownReportIgnored(t *testing.T) {
	tracker := NewTracker(time.Minute, func(ev DeliveryEvent) {
		t.Errorf("unexpected event for unknown report: %+v", ev)
	})

	// No Track call — must be a no-op, must not panic.
	tracker.HandleReport(&pdu.StatusReport{
		MessageRef: 7,
		Recipient:  "+79991112233",
		Delivered:  true,
		StatusCode: 0x00,
	})
}

func TestTracker_RecipientFormatMismatch(t *testing.T) {
	var mu sync.Mutex
	var events []DeliveryEvent

	tracker := NewTracker(time.Minute, func(ev DeliveryEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	})

	// Track normalized E.164, report arrives with national format (no "+",
	// leading 8) as sent by some networks with TOA 0x81.
	tracker.Track(21, "+79991112233", "siemens_tc35")
	tracker.HandleReport(&pdu.StatusReport{
		MessageRef: 21,
		Recipient:  "89991112233",
		Delivered:  true,
		StatusCode: 0x00,
	})

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Fatalf("expected 2 events (pending, delivered), got %d", len(events))
	}
	if events[1].Status != DeliveryStatusDelivered {
		t.Errorf("expected delivered, got %v", events[1].Status)
	}
}

func TestTracker_SameRefDifferentRecipients(t *testing.T) {
	var mu sync.Mutex
	var delivered int

	tracker := NewTracker(time.Minute, func(ev DeliveryEvent) {
		if ev.Status == DeliveryStatusDelivered {
			mu.Lock()
			delivered++
			mu.Unlock()
		}
	})

	tracker.Track(30, "+79991112233", "modem1")
	tracker.Track(30, "+79992223344", "modem1")

	tracker.HandleReport(&pdu.StatusReport{
		MessageRef: 30,
		Recipient:  "+79991112233",
		Delivered:  true,
		StatusCode: 0x00,
	})

	// Read the counter without holding the lock across tracker calls:
	// HandleReport invokes onUpdate synchronously, which takes mu.
	mu.Lock()
	first := delivered
	mu.Unlock()
	if first != 1 {
		t.Errorf("expected exactly 1 delivered event, got %d", first)
	}

	// Second recipient must still be pending (no timeout yet, no report).
	tracker.HandleReport(&pdu.StatusReport{
		MessageRef: 30,
		Recipient:  "+79992223344",
		Delivered:  true,
		StatusCode: 0x00,
	})
	mu.Lock()
	defer mu.Unlock()
	if delivered != 2 {
		t.Errorf("expected 2 delivered events after second report, got %d", delivered)
	}
}

func TestTracker_OverwriteStopsOldTimer(t *testing.T) {
	var mu sync.Mutex
	var expired int

	tracker := NewTracker(50*time.Millisecond, func(ev DeliveryEvent) {
		if ev.Status == DeliveryStatusExpired {
			mu.Lock()
			expired++
			mu.Unlock()
		}
	})

	tracker.Track(40, "+79991112233", "modem1")
	time.Sleep(20 * time.Millisecond)
	tracker.Track(40, "+79991112233", "modem1") // overwrites, restarts timer
	time.Sleep(30 * time.Millisecond)           // first timer would have fired here

	mu.Lock()
	first := expired
	mu.Unlock()
	if first != 0 {
		t.Fatalf("old timer fired after overwrite: expired=%d", first)
	}

	time.Sleep(60 * time.Millisecond) // second timer fires
	mu.Lock()
	defer mu.Unlock()
	if expired != 1 {
		t.Errorf("expected exactly 1 expiry after overwrite, got %d", expired)
	}
}
