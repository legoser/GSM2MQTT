package mqtt

import (
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
	SwVersion    string   `json:"sw_version,omitempty"`
	ViaDevice    string   `json:"via_device,omitempty"`
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

func buildAvailability(topicPrefix string) (string, string, string) {
	return fmt.Sprintf("%s/status", topicPrefix), "online", "offline"
}

// BuildSignalDiscovery generates the MQTT Auto-Discovery configuration for GSM signal strength sensor.
func BuildSignalDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("signal")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/signal", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Signal Strength",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("signal"),
		StateTopic:          stateTopic,
		UnitOfMeasurement:   "dBm",
		DeviceClass:         "signal_strength",
		StateClass:          "measurement",
		ValueTemplate:       "{{ value_json.dbm }}",
		Icon:                "mdi:signal-cellular-3",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildBalanceDiscovery generates discovery for SIM card monetary balance sensor.
func BuildBalanceDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("balance")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/balance", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	currency := p.Currency
	if currency == "" {
		currency = "RUB"
	}

	payload := SensorDiscoveryPayload{
		Name:                "Balance",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("balance"),
		StateTopic:          stateTopic,
		UnitOfMeasurement:   currency,
		DeviceClass:         "monetary",
		Icon:                "mdi:cash",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildStatusDiscovery generates discovery for modem operational health status sensor.
func BuildStatusDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("status")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/health", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Status",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("status"),
		StateTopic:          stateTopic,
		JSONAttributesTopic: stateTopic,
		DeviceClass:         "enum",
		Options:             []string{"ready", "degraded", "not_ready", "error", "disconnected"},
		ValueTemplate:       "{{ value_json.status }}",
		Icon:                "mdi:chip",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildOperatorDiscovery generates discovery for network operator name sensor.
func BuildOperatorDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("operator")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/health", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Operator",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("operator"),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.operator }}",
		Icon:                "mdi:cellphone-tower",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildLastSMSDiscovery generates discovery for last incoming SMS sensor.
func BuildLastSMSDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("last_sms")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/sms/received", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Last Incoming SMS",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("last_sms"),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.text }}",
		JSONAttributesTopic: stateTopic,
		Icon:                "mdi:message-text",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildUSSDResponseDiscovery generates discovery for USSD response text sensor.
func BuildUSSDResponseDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("ussd_response")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/ussd/response", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "USSD Response",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("ussd_response"),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.message }}",
		Icon:                "mdi:phone-incoming",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}
