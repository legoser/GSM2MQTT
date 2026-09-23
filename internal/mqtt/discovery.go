package mqtt

import (
	"fmt"
	"strings"
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
	Name                string      `json:"name"`
	UniqueID            string      `json:"unique_id"`
	ObjectID            string      `json:"object_id,omitempty"`
	StateTopic          string      `json:"state_topic"`
	UnitOfMeasurement   string      `json:"unit_of_measurement,omitempty"`
	DeviceClass         string      `json:"device_class,omitempty"`
	StateClass          string      `json:"state_class,omitempty"`
	ValueTemplate       string      `json:"value_template,omitempty"`
	JSONAttributesTopic string      `json:"json_attributes_topic,omitempty"`
	Icon                string      `json:"icon,omitempty"`
	Options             []string    `json:"options,omitempty"`
	AvailabilityTopic   string      `json:"availability_topic,omitempty"`
	PayloadAvailable    string      `json:"payload_available,omitempty"`
	PayloadNotAvailable string      `json:"payload_not_available,omitempty"`
	Device              *DeviceInfo `json:"device"`
}

// ButtonDiscoveryPayload represents Home Assistant MQTT button configuration.
type ButtonDiscoveryPayload struct {
	Name                string      `json:"name"`
	UniqueID            string      `json:"unique_id"`
	ObjectID            string      `json:"object_id,omitempty"`
	CommandTopic        string      `json:"command_topic"`
	PayloadPress        string      `json:"payload_press,omitempty"`
	Icon                string      `json:"icon,omitempty"`
	AvailabilityTopic   string      `json:"availability_topic,omitempty"`
	PayloadAvailable    string      `json:"payload_available,omitempty"`
	PayloadNotAvailable string      `json:"payload_not_available,omitempty"`
	Device              *DeviceInfo `json:"device"`
}

// BinarySensorDiscoveryPayload represents Home Assistant MQTT binary sensor configuration.
type BinarySensorDiscoveryPayload struct {
	Name                string      `json:"name"`
	UniqueID            string      `json:"unique_id"`
	ObjectID            string      `json:"object_id,omitempty"`
	StateTopic          string      `json:"state_topic"`
	ValueTemplate       string      `json:"value_template,omitempty"`
	PayloadOn           string      `json:"payload_on,omitempty"`
	OffDelay            int         `json:"off_delay,omitempty"`
	Icon                string      `json:"icon,omitempty"`
	AvailabilityTopic   string      `json:"availability_topic,omitempty"`
	PayloadAvailable    string      `json:"payload_available,omitempty"`
	PayloadNotAvailable string      `json:"payload_not_available,omitempty"`
	Device              *DeviceInfo `json:"device"`
}

func sanitizeDevice(modemID, manufacturer, model string) (cleanMfg, cleanModel, deviceName string) {
	cleanMfg = strings.TrimSpace(manufacturer)
	cleanModel = strings.TrimSpace(model)

	if strings.EqualFold(cleanMfg, "undefined") || cleanMfg == "" {
		cleanMfg = "Unknown"
	}

	if cleanModel == "" {
		cleanModel = "Modem"
	}

	if cleanMfg == "Unknown" && cleanModel == "Modem" {
		deviceName = fmt.Sprintf("GSM Modem (%s)", modemID)
	} else {
		deviceName = fmt.Sprintf("%s %s", cleanMfg, cleanModel)
	}
	return cleanMfg, cleanModel, deviceName
}

func buildDeviceInfo(modemID, manufacturer, model string) *DeviceInfo {
	cleanMfg, cleanModel, deviceName := sanitizeDevice(modemID, manufacturer, model)
	return &DeviceInfo{
		Identifiers:  []string{fmt.Sprintf("gsm2mqtt_%s", modemID)},
		Name:         deviceName,
		Manufacturer: cleanMfg,
		Model:        cleanModel,
	}
}

func buildAvailability(topicPrefix string) (string, string, string) {
	return fmt.Sprintf("%s/status", topicPrefix), "online", "offline"
}

// BuildSignalDiscovery generates the MQTT Auto-Discovery configuration for GSM signal strength sensor.
func BuildSignalDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_signal", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/signal", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Signal Strength",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_signal", modemID),
		StateTopic:          stateTopic,
		UnitOfMeasurement:   "dBm",
		DeviceClass:         "signal_strength",
		StateClass:          "measurement",
		ValueTemplate:       "{{ value_json.dbm }}",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildBalanceDiscovery generates discovery for SIM card monetary balance sensor.
func BuildBalanceDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model, currency string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_balance", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/balance", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	if currency == "" {
		currency = "RUB"
	}

	payload := SensorDiscoveryPayload{
		Name:                "Balance",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_balance", modemID),
		StateTopic:          stateTopic,
		UnitOfMeasurement:   currency,
		DeviceClass:         "monetary",
		Icon:                "mdi:cash",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildStatusDiscovery generates discovery for modem operational health status sensor.
func BuildStatusDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_status", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/health", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Status",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_status", modemID),
		StateTopic:          stateTopic,
		DeviceClass:         "enum",
		Options:             []string{"ready", "degraded", "not_ready", "error"},
		ValueTemplate:       "{{ value_json.status }}",
		Icon:                "mdi:chip",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildOperatorDiscovery generates discovery for network operator name sensor.
func BuildOperatorDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_operator", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/health", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Operator",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_operator", modemID),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.operator }}",
		Icon:                "mdi:cellphone-tower",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildLastSMSDiscovery generates discovery for last incoming SMS sensor.
func BuildLastSMSDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_last_sms", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/sms/received", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Last Incoming SMS",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_last_sms", modemID),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.text }}",
		JSONAttributesTopic: stateTopic,
		Icon:                "mdi:message-text",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildUSSDResponseDiscovery generates discovery for USSD response text sensor.
func BuildUSSDResponseDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_ussd_response", modemID)
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/ussd/response", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "USSD Response",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_ussd_response", modemID),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.message }}",
		Icon:                "mdi:phone-incoming",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

