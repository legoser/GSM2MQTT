package services

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent_PayloadStructure(t *testing.T) {
	loc := time.FixedZone("UTC+03:00", 3*3600)
	evt := NewEvent(
		"modem_1",
		"low_balance",
		EventCategoryTariff,
		EventLevelWarning,
		"Balance is low: 3.22 RUB (threshold: 5.00 RUB)",
		loc,
		map[string]any{
			"balance":   3.22,
			"threshold": 5.0,
			"currency":  "RUB",
		},
	)

	if evt.EventType != "low_balance" {
		t.Errorf("expected EventType = 'low_balance', got %s", evt.EventType)
	}
	if evt.Event != "low_balance" {
		t.Errorf("expected Event = 'low_balance', got %s", evt.Event)
	}
	if evt.Category != "tariff" {
		t.Errorf("expected Category = 'tariff', got %s", evt.Category)
	}
	if evt.Level != "warning" {
		t.Errorf("expected Level = 'warning', got %s", evt.Level)
	}
	if evt.ModemID != "modem_1" {
		t.Errorf("expected ModemID = 'modem_1', got %s", evt.ModemID)
	}

	data, err := evt.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if parsed["event_type"] != "low_balance" {
		t.Errorf("expected JSON event_type = 'low_balance', got %v", parsed["event_type"])
	}
	if parsed["category"] != "tariff" {
		t.Errorf("expected JSON category = 'tariff', got %v", parsed["category"])
	}
}
