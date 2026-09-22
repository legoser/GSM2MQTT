package mqtt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSignalDiscovery(t *testing.T) {
	msg, err := BuildSignalDiscovery("homeassistant", "gsm2mqtt", "siemens_tc35", "Siemens", "TC35")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil discovery message")
	}

	expectedTopic := "homeassistant/sensor/gsm2mqtt_siemens_tc35_signal/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, msg.Topic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	if payload["device_class"] != "signal_strength" {
		t.Errorf("expected device_class 'signal_strength', got %v", payload["device_class"])
	}
	if payload["unit_of_measurement"] != "dBm" {
		t.Errorf("expected unit 'dBm', got %v", payload["unit_of_measurement"])
	}
	expectedStateTopic := "gsm2mqtt/modem/siemens_tc35/signal"
	if payload["state_topic"] != expectedStateTopic {
		t.Errorf("expected state_topic %q, got %v", expectedStateTopic, payload["state_topic"])
	}

	devRaw, ok := payload["device"].(map[string]interface{})
	if !ok {
		t.Fatal("expected device dictionary in discovery payload")
	}
	if !strings.Contains(devRaw["name"].(string), "Siemens") {
		t.Errorf("expected device name to mention 'Siemens', got %v", devRaw["name"])
	}
}
