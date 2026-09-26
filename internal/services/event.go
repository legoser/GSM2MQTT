// Package services implements core GSM gateway business logic, background workers, and telemetry routing.
package services

import (
	"encoding/json"
	"time"

	"github.com/legoser/gsm2mqtt/internal/system"
)

// Event categories
const (
	EventCategoryTariff   = "tariff"
	EventCategoryHardware = "hardware"
	EventCategoryNetwork  = "network"
	EventCategorySMS      = "sms"
)

// Event severity levels
const (
	EventLevelInfo     = "info"
	EventLevelWarning  = "warning"
	EventLevelError    = "error"
	EventLevelCritical = "critical"
)

// EventPayload represents a structured event emitted by the modem runner.
// It includes EventType for Home Assistant MQTT Event entity compatibility.
type EventPayload struct {
	EventType string         `json:"event_type"`     // Matches Home Assistant event_type
	Event     string         `json:"event"`          // Generic alias
	Category  string         `json:"category"`       // tariff, hardware, network, sms
	Level     string         `json:"level"`          // info, warning, error, critical
	Message   string         `json:"message"`        // Human-readable summary
	Timestamp string         `json:"timestamp"`      // ISO 8601 with timezone offset
	ModemID   string         `json:"modem_id"`       // ID of originating modem
	Data      map[string]any `json:"data,omitempty"` // Contextual telemetry
}

// NewEvent constructs an EventPayload with populated timestamp and normalized event types.
func NewEvent(modemID, eventType, category, level, message string, loc *time.Location, data map[string]any) EventPayload {
	return EventPayload{
		EventType: eventType,
		Event:     eventType,
		Category:  category,
		Level:     level,
		Message:   message,
		Timestamp: system.FormatLocalTime(time.Now(), loc),
		ModemID:   modemID,
		Data:      data,
	}
}

// Marshal returns the JSON encoding of the event.
func (e EventPayload) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
