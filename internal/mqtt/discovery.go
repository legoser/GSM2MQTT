package mqtt

import (
	"encoding/json"
	"fmt"
)

// DiscoveryMessage represents a Home Assistant MQTT Auto Discovery payload.
type DiscoveryMessage struct {
	Topic   string
	Payload []byte
}

// DeviceInfo represents Home Assistant device metadata.
type DeviceInfo struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
}

// SensorDiscoveryPayload represents Home Assistant MQTT sensor configuration.
type SensorDiscoveryPayload struct {
	Name              string      `json:"name"`
	UniqueID          string      `json:"unique_id"`
	StateTopic        string      `json:"state_topic"`
	UnitOfMeasurement string      `json:"unit_of_measurement,omitempty"`
	DeviceClass       string      `json:"device_class,omitempty"`
	StateClass        string      `json:"state_class,omitempty"`
	Device            *DeviceInfo `json:"device"`
}

// BuildSignalDiscovery generates the MQTT Auto-Discovery configuration for GSM signal strength sensor.
func BuildSignalDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_signal", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/signal", topicPrefix, modemID)

	deviceName := fmt.Sprintf("%s %s", manufacturer, model)
	if manufacturer == "" && model == "" {
		deviceName = fmt.Sprintf("GSM Modem %s", modemID)
	}

	payload := SensorDiscoveryPayload{
		Name:              "Signal Strength",
		UniqueID:          uniqueID,
		StateTopic:        stateTopic,
		UnitOfMeasurement: "dBm",
		DeviceClass:       "signal_strength",
		StateClass:        "measurement",
		Device: &DeviceInfo{
			Identifiers:  []string{fmt.Sprintf("gsm2mqtt_%s", modemID)},
			Name:         deviceName,
			Manufacturer: manufacturer,
			Model:        model,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signal discovery: %w", err)
	}

	return &DiscoveryMessage{
		Topic:   topic,
		Payload: data,
	}, nil
}
