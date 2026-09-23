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
	if payload["value_template"] != "{{ value_json.dbm }}" {
		t.Errorf("expected value_template '{{ value_json.dbm }}', got %v", payload["value_template"])
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

func TestBuildModemDiscoveries(t *testing.T) {
	msgs, err := BuildModemDiscoveries("homeassistant", "gsm2mqtt", "neoway_m590", "Undefined", "M590", "RUB")
	if err != nil {
		t.Fatalf("BuildModemDiscoveries failed: %v", err)
	}
	if len(msgs) != 10 {
		t.Fatalf("expected 10 discovery messages, got %d", len(msgs))
	}

	expectedIDs := map[string]bool{
		"gsm2mqtt_neoway_m590_signal":        false,
		"gsm2mqtt_neoway_m590_balance":       false,
		"gsm2mqtt_neoway_m590_status":        false,
		"gsm2mqtt_neoway_m590_operator":      false,
		"gsm2mqtt_neoway_m590_last_sms":      false,
		"gsm2mqtt_neoway_m590_ussd_response": false,
		"gsm2mqtt_neoway_m590_btn_balance":   false,
		"gsm2mqtt_neoway_m590_btn_hangup":    false,
		"gsm2mqtt_neoway_m590_incoming_call": false,
		"gsm2mqtt_neoway_m590_new_sms":       false,
	}

	for _, msg := range msgs {
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("failed to unmarshal discovery payload for %s: %v", msg.Topic, err)
		}
		uID, ok := payload["unique_id"].(string)
		if !ok {
			t.Fatalf("missing unique_id in payload for %s", msg.Topic)
		}
		if _, exists := expectedIDs[uID]; !exists {
			t.Errorf("unexpected unique_id: %s", uID)
		}
		expectedIDs[uID] = true

		dev, ok := payload["device"].(map[string]interface{})
		if !ok {
			t.Fatalf("missing device object for %s", msg.Topic)
		}
		mfg, _ := dev["manufacturer"].(string)
		if mfg != "Unknown" {
			t.Errorf("expected sanitized manufacturer 'Unknown', got %q", mfg)
		}
		devName, _ := dev["name"].(string)
		if devName != "Unknown M590" {
			t.Errorf("expected device name 'Unknown M590', got %q", devName)
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected discovery message for %s was not produced", id)
		}
	}
}
