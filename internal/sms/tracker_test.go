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
