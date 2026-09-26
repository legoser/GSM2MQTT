package mqtt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildGatewayDiscovery(t *testing.T) {
	msg, err := BuildGatewayDiscovery("homeassistant", "gsm2mqtt", "1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedTopic := "homeassistant/sensor/gsm2mqtt_gateway_status/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, msg.Topic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	if payload["unique_id"] != "gsm2mqtt_gateway_status" {
		t.Errorf("expected unique_id gsm2mqtt_gateway_status, got %v", payload["unique_id"])
	}

	devRaw, ok := payload["device"].(map[string]interface{})
	if !ok {
		t.Fatal("expected device dictionary in gateway discovery")
	}
	if devRaw["name"] != "GSM2MQTT Gateway" {
		t.Errorf("expected device name 'GSM2MQTT Gateway', got %v", devRaw["name"])
	}
	if devRaw["sw_version"] != "1.2.0" {
		t.Errorf("expected sw_version '1.2.0', got %v", devRaw["sw_version"])
	}
	ids, ok := devRaw["identifiers"].([]interface{})
	if !ok || len(ids) == 0 || ids[0] != "gsm2mqtt_gateway" {
		t.Errorf("expected identifier 'gsm2mqtt_gateway', got %v", ids)
	}

	// Test default version fallback to version.Version
	msgDef, err := BuildGatewayDiscovery("homeassistant", "gsm2mqtt", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payloadDef map[string]interface{}
	if err := json.Unmarshal(msgDef.Payload, &payloadDef); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}
	devDef := payloadDef["device"].(map[string]interface{})
	if devDef["sw_version"] == "" || devDef["sw_version"] == "1.0.0" {
		t.Errorf("expected sw_version to come from version package, got %v", devDef["sw_version"])
	}
}

func TestBuildGatewayModemSensorsDiscovery(t *testing.T) {
	mcMsg, err := BuildGatewayModemCountDiscovery("homeassistant", "gsm2mqtt", "1.2.0")
	if err != nil {
		t.Fatalf("unexpected error for modem count: %v", err)
	}
	if mcMsg.Topic != "homeassistant/sensor/gsm2mqtt_gateway_modem_count/config" {
		t.Errorf("unexpected topic %s", mcMsg.Topic)
	}

	amMsg, err := BuildGatewayActiveModemDiscovery("homeassistant", "gsm2mqtt", "1.2.0")
	if err != nil {
		t.Fatalf("unexpected error for active modem: %v", err)
	}
	if amMsg.Topic != "homeassistant/sensor/gsm2mqtt_gateway_active_modem/config" {
		t.Errorf("unexpected topic %s", amMsg.Topic)
	}
}

func TestBuildRecipientsTextDiscovery(t *testing.T) {
	msg, err := BuildRecipientsTextDiscovery("homeassistant", "gsm2mqtt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedTopic := "homeassistant/text/gsm2mqtt_gateway_recipients/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, msg.Topic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	if payload["unique_id"] != "gsm2mqtt_gateway_recipients" {
		t.Errorf("expected unique_id gsm2mqtt_gateway_recipients, got %v", payload["unique_id"])
	}
	if payload["command_topic"] != "gsm2mqtt/config/recipients/set" {
		t.Errorf("expected command_topic gsm2mqtt/config/recipients/set, got %v", payload["command_topic"])
	}
	if payload["state_topic"] != "gsm2mqtt/config/recipients" {
		t.Errorf("expected state_topic gsm2mqtt/config/recipients, got %v", payload["state_topic"])
	}
}

func TestBuildNotifyDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "siemens_mc35i",
		SlotIndex:       1,
	}
	msg, err := BuildNotifyDiscovery(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTopic := "homeassistant/notify/gsm2mqtt_modem_1_notify/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, msg.Topic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	if payload["unique_id"] != "gsm2mqtt_modem_1_notify" {
		t.Errorf("expected unique_id gsm2mqtt_modem_1_notify, got %v", payload["unique_id"])
	}
	if payload["object_id"] != "gsm_modem_notify" {
		t.Errorf("expected object_id gsm_modem_notify, got %v", payload["object_id"])
	}
	if payload["command_topic"] != "gsm2mqtt/modem/gsm_modem/sms/send" {
		t.Errorf("expected command_topic gsm2mqtt/modem/gsm_modem/sms/send, got %v", payload["command_topic"])
	}
	expectedTemplate := `{"text": {{ value | tojson }}}`
	if payload["command_template"] != expectedTemplate {
		t.Errorf("expected command_template %q, got %v", expectedTemplate, payload["command_template"])
	}
}

func TestBuildSignalDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "siemens_tc35",
		Manufacturer:    "Siemens",
		Model:           "TC35",
		SlotIndex:       1,
	}
	msg, err := BuildSignalDiscovery(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil discovery message")
	}

	expectedTopic := "homeassistant/sensor/gsm2mqtt_modem_1_signal/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, msg.Topic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	if payload["unique_id"] != "gsm2mqtt_modem_1_signal" {
		t.Errorf("expected unique_id gsm2mqtt_modem_1_signal, got %v", payload["unique_id"])
	}
	if payload["object_id"] != "gsm_modem_signal" {
		t.Errorf("expected object_id gsm_modem_signal, got %v", payload["object_id"])
	}

	devRaw, ok := payload["device"].(map[string]interface{})
	if !ok {
		t.Fatal("expected device dictionary in discovery payload")
	}
	if devRaw["name"] != "GSM Modem" {
		t.Errorf("expected device name 'GSM Modem', got %v", devRaw["name"])
	}
	if devRaw["via_device"] != "gsm2mqtt_gateway" {
		t.Errorf("expected via_device 'gsm2mqtt_gateway', got %v", devRaw["via_device"])
	}
}

func TestBuildLastSMSDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "siemens_tc35",
		SlotIndex:       1,
	}
	msg, err := BuildLastSMSDiscovery(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if payload["state_topic"] != "gsm2mqtt/modem/siemens_tc35/sms/last" {
		t.Errorf("expected state_topic gsm2mqtt/modem/siemens_tc35/sms/last, got %v", payload["state_topic"])
	}
}

func TestBuildIncomingCallDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "siemens_tc35",
		SlotIndex:       1,
	}
	msg, err := BuildIncomingCallDiscovery(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if payload["payload_on"] != "ON" || payload["payload_off"] != "OFF" {
		t.Errorf("expected payload_on='ON' and payload_off='OFF', got %v / %v", payload["payload_on"], payload["payload_off"])
	}
	if payload["state_topic"] != "gsm2mqtt/modem/siemens_tc35/call/incoming" {
		t.Errorf("expected state_topic gsm2mqtt/modem/siemens_tc35/call/incoming, got %v", payload["state_topic"])
	}
}

func TestBuildCallerNumberDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "siemens_tc35",
		SlotIndex:       1,
	}
	msg, err := BuildCallerNumberDiscovery(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if payload["unique_id"] != "gsm2mqtt_modem_1_caller_number" {
		t.Errorf("expected unique_id gsm2mqtt_modem_1_caller_number, got %v", payload["unique_id"])
	}
	if payload["state_topic"] != "gsm2mqtt/modem/siemens_tc35/call/incoming" {
		t.Errorf("expected state_topic gsm2mqtt/modem/siemens_tc35/call/incoming, got %v", payload["state_topic"])
	}
}

func TestBuildModemDiscoveries_OptionB_Slot1(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "neoway_m590",
		Manufacturer:    "Undefined",
		Model:           "M590",
		Currency:        "RUB",
		SlotIndex:       1,
	}
	msgs, err := BuildModemDiscoveries(params)
	if err != nil {
		t.Fatalf("BuildModemDiscoveries failed: %v", err)
	}
	if len(msgs) != 24 {
		t.Fatalf("expected 24 discovery messages, got %d", len(msgs))
	}

	expectedIDs := map[string]bool{
		"gsm2mqtt_modem_1_signal":                 false,
		"gsm2mqtt_modem_1_balance":                false,
		"gsm2mqtt_modem_1_status":                 false,
		"gsm2mqtt_modem_1_connected":              false,
		"gsm2mqtt_modem_1_problem":                false,
		"gsm2mqtt_modem_1_events":                 false,
		"gsm2mqtt_modem_1_operator":               false,
		"gsm2mqtt_modem_1_last_sms":               false,
		"gsm2mqtt_modem_1_ussd_response":          false,
		"gsm2mqtt_modem_1_btn_balance":            false,
		"gsm2mqtt_modem_1_btn_hangup":             false,
		"gsm2mqtt_modem_1_incoming_call":          false,
		"gsm2mqtt_modem_1_caller_number":          false,
		"gsm2mqtt_modem_1_new_sms":                false,
		"gsm2mqtt_modem_1_notify":                 false,
		"gsm2mqtt_modem_1_sms_remaining":          false,
		"gsm2mqtt_modem_1_call_minutes_remaining": false,
		"gsm2mqtt_modem_1_btn_tariff_reset":       false,
		"gsm2mqtt_modem_1_data_traffic_remaining": false,
		"gsm2mqtt_modem_1_low_balance":            false,
		"gsm2mqtt_modem_1_sms_history":            false,
		"gsm2mqtt_modem_1_btn_clear_sms_history":  false,
		"gsm2mqtt_modem_1_call_history":           false,
		"gsm2mqtt_modem_1_btn_clear_call_history": false,
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
		if devName != "GSM Modem" {
			t.Errorf("expected device name 'GSM Modem', got %q", devName)
		}
		viaDev, _ := dev["via_device"].(string)
		if viaDev != "gsm2mqtt_gateway" {
			t.Errorf("expected via_device 'gsm2mqtt_gateway', got %q", viaDev)
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected discovery message for %s was not produced", id)
		}
	}
}

func TestBuildModemDiscoveries_OptionB_Slot2(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "siemens_mc35i",
		Manufacturer:    "SIEMENS",
		Model:           "MC35i",
		Currency:        "RUB",
		SlotIndex:       2,
	}
	msgs, err := BuildModemDiscoveries(params)
	if err != nil {
		t.Fatalf("BuildModemDiscoveries failed: %v", err)
	}

	for _, msg := range msgs {
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("failed to unmarshal discovery payload: %v", err)
		}
		uID := payload["unique_id"].(string)
		if !strings.HasPrefix(uID, "gsm2mqtt_modem_2_") {
			t.Errorf("expected slot 2 prefix 'gsm2mqtt_modem_2_', got %s", uID)
		}
		oID := payload["object_id"].(string)
		if !strings.HasPrefix(oID, "gsm_modem_2_") {
			t.Errorf("expected slot 2 object_id prefix 'gsm_modem_2_', got %s", oID)
		}
		dev := payload["device"].(map[string]interface{})
		if dev["name"] != "GSM Modem 2" {
			t.Errorf("expected device name 'GSM Modem 2', got %v", dev["name"])
		}
	}
}

func TestBuildConnectedBinaryDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "modem1",
		SlotIndex:       1,
	}
	msg, err := BuildConnectedBinaryDiscovery(params)
	if err != nil {
		t.Fatalf("BuildConnectedBinaryDiscovery failed: %v", err)
	}
	expectedTopic := "homeassistant/binary_sensor/gsm2mqtt_modem_1_connected/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %s, got %s", expectedTopic, msg.Topic)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["device_class"] != "connectivity" {
		t.Errorf("expected device_class 'connectivity', got %v", payload["device_class"])
	}
}

func TestBuildProblemBinaryDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "modem1",
		SlotIndex:       1,
	}
	msg, err := BuildProblemBinaryDiscovery(params)
	if err != nil {
		t.Fatalf("BuildProblemBinaryDiscovery failed: %v", err)
	}
	expectedTopic := "homeassistant/binary_sensor/gsm2mqtt_modem_1_problem/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %s, got %s", expectedTopic, msg.Topic)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["device_class"] != "problem" {
		t.Errorf("expected device_class 'problem', got %v", payload["device_class"])
	}
}

func TestBuildEventsDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "modem1",
		SlotIndex:       1,
	}
	msg, err := BuildEventsDiscovery(params)
	if err != nil {
		t.Fatalf("BuildEventsDiscovery failed: %v", err)
	}
	expectedTopic := "homeassistant/event/gsm2mqtt_modem_1_events/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %s, got %s", expectedTopic, msg.Topic)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	eventTypes, ok := payload["event_types"].([]interface{})
	if !ok || len(eventTypes) == 0 {
		t.Errorf("expected non-empty event_types array, got %v", payload["event_types"])
	}
}

func TestBuildTariffLowBalanceBinaryDiscovery(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "modem1",
		SlotIndex:       1,
	}
	msg, err := BuildTariffLowBalanceBinaryDiscovery(params)
	if err != nil {
		t.Fatalf("BuildTariffLowBalanceBinaryDiscovery failed: %v", err)
	}
	expectedTopic := "homeassistant/binary_sensor/gsm2mqtt_modem_1_low_balance/config"
	if msg.Topic != expectedTopic {
		t.Errorf("expected topic %s, got %s", expectedTopic, msg.Topic)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["device_class"] != "problem" {
		t.Errorf("expected device_class 'problem', got %v", payload["device_class"])
	}
}

func TestBuildHistoryDiscoveries(t *testing.T) {
	params := ModemDiscoveryParams{
		DiscoveryPrefix: "homeassistant",
		TopicPrefix:     "gsm2mqtt",
		ModemID:         "modem1",
		SlotIndex:       1,
	}

	// 1. SMS History sensor
	smsMsg, err := BuildSMSHistoryDiscovery(params)
	if err != nil {
		t.Fatalf("BuildSMSHistoryDiscovery failed: %v", err)
	}
	if smsMsg.Topic != "homeassistant/sensor/gsm2mqtt_modem_1_sms_history/config" {
		t.Errorf("unexpected topic: %s", smsMsg.Topic)
	}
	var smsPayload map[string]interface{}
	if err := json.Unmarshal(smsMsg.Payload, &smsPayload); err != nil {
		t.Fatal(err)
	}
	if smsPayload["state_topic"] != "gsm2mqtt/modem/modem1/sms/history" {
		t.Errorf("expected state_topic 'gsm2mqtt/modem/modem1/sms/history', got %v", smsPayload["state_topic"])
	}
	if smsPayload["json_attributes_topic"] != "gsm2mqtt/modem/modem1/sms/history" {
		t.Errorf("expected json_attributes_topic 'gsm2mqtt/modem/modem1/sms/history', got %v", smsPayload["json_attributes_topic"])
	}

	// 2. Clear SMS History button
	clearSMSMsg, err := BuildClearSMSHistoryButtonDiscovery(params)
	if err != nil {
		t.Fatalf("BuildClearSMSHistoryButtonDiscovery failed: %v", err)
	}
	if clearSMSMsg.Topic != "homeassistant/button/gsm2mqtt_modem_1_btn_clear_sms_history/config" {
		t.Errorf("unexpected topic: %s", clearSMSMsg.Topic)
	}
	var clearSMSPayload map[string]interface{}
	if err := json.Unmarshal(clearSMSMsg.Payload, &clearSMSPayload); err != nil {
		t.Fatal(err)
	}
	if clearSMSPayload["command_topic"] != "gsm2mqtt/modem/modem1/sms/history/clear" {
		t.Errorf("expected command_topic 'gsm2mqtt/modem/modem1/sms/history/clear', got %v", clearSMSPayload["command_topic"])
	}

	// 3. Call History sensor
	callMsg, err := BuildCallHistoryDiscovery(params)
	if err != nil {
		t.Fatalf("BuildCallHistoryDiscovery failed: %v", err)
	}
	if callMsg.Topic != "homeassistant/sensor/gsm2mqtt_modem_1_call_history/config" {
		t.Errorf("unexpected topic: %s", callMsg.Topic)
	}
	var callPayload map[string]interface{}
	if err := json.Unmarshal(callMsg.Payload, &callPayload); err != nil {
		t.Fatal(err)
	}
	if callPayload["state_topic"] != "gsm2mqtt/modem/modem1/call/history" {
		t.Errorf("expected state_topic 'gsm2mqtt/modem/modem1/call/history', got %v", callPayload["state_topic"])
	}
	if callPayload["json_attributes_topic"] != "gsm2mqtt/modem/modem1/call/history" {
		t.Errorf("expected json_attributes_topic 'gsm2mqtt/modem/modem1/call/history', got %v", callPayload["json_attributes_topic"])
	}

	// 4. Clear Call History button
	clearCallMsg, err := BuildClearCallHistoryButtonDiscovery(params)
	if err != nil {
		t.Fatalf("BuildClearCallHistoryButtonDiscovery failed: %v", err)
	}
	if clearCallMsg.Topic != "homeassistant/button/gsm2mqtt_modem_1_btn_clear_call_history/config" {
		t.Errorf("unexpected topic: %s", clearCallMsg.Topic)
	}
	var clearCallPayload map[string]interface{}
	if err := json.Unmarshal(clearCallMsg.Payload, &clearCallPayload); err != nil {
		t.Fatal(err)
	}
	if clearCallPayload["command_topic"] != "gsm2mqtt/modem/modem1/call/history/clear" {
		t.Errorf("expected command_topic 'gsm2mqtt/modem/modem1/call/history/clear', got %v", clearCallPayload["command_topic"])
	}
}
