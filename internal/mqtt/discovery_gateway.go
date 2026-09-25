package mqtt

import (
	"fmt"
	"strings"

	"github.com/legoser/gsm2mqtt/internal/version"
)

// GatewayIdentifier is the canonical identifier for the parent GSM2MQTT Gateway device in Home Assistant.
const GatewayIdentifier = "gsm2mqtt_gateway"

// ModemDiscoveryParams encapsulates parameters required to generate Home Assistant Auto-Discovery messages for a modem.
type ModemDiscoveryParams struct {
	DiscoveryPrefix string
	TopicPrefix     string
	ModemID         string
	Manufacturer    string
	Model           string
	SwVersion       string
	Currency        string
	SlotIndex       int
}

// DeviceName returns the standard Home Assistant device name following Option B.
func (p ModemDiscoveryParams) DeviceName() string {
	if p.SlotIndex <= 1 {
		return "GSM Modem"
	}
	return fmt.Sprintf("GSM Modem %d", p.SlotIndex)
}

// DeviceIdentifier returns the stable device identifier for Home Assistant.
func (p ModemDiscoveryParams) DeviceIdentifier() string {
	if p.SlotIndex <= 1 {
		return "gsm2mqtt_modem_1"
	}
	return fmt.Sprintf("gsm2mqtt_modem_%d", p.SlotIndex)
}

// EntityUniqueID returns a unique entity identifier across devices and slots.
func (p ModemDiscoveryParams) EntityUniqueID(metric string) string {
	if p.SlotIndex <= 1 {
		return fmt.Sprintf("gsm2mqtt_modem_1_%s", metric)
	}
	return fmt.Sprintf("gsm2mqtt_modem_%d_%s", p.SlotIndex, metric)
}

// EntityObjectID returns the Home Assistant entity object ID (which drives the entity_id).
func (p ModemDiscoveryParams) EntityObjectID(metric string) string {
	if p.SlotIndex <= 1 {
		return fmt.Sprintf("gsm_modem_%s", metric)
	}
	return fmt.Sprintf("gsm_modem_%d_%s", p.SlotIndex, metric)
}

// TextDiscoveryPayload represents Home Assistant MQTT text entity configuration.
type TextDiscoveryPayload struct {
	Name                string      `json:"name"`
	UniqueID            string      `json:"unique_id"`
	ObjectID            string      `json:"object_id,omitempty"`
	CommandTopic        string      `json:"command_topic"`
	StateTopic          string      `json:"state_topic"`
	CommandTemplate     string      `json:"command_template,omitempty"`
	ValueTemplate       string      `json:"value_template,omitempty"`
	Icon                string      `json:"icon,omitempty"`
	AvailabilityTopic   string      `json:"availability_topic,omitempty"`
	PayloadAvailable    string      `json:"payload_available,omitempty"`
	PayloadNotAvailable string      `json:"payload_not_available,omitempty"`
	Device              *DeviceInfo `json:"device"`
}

// BuildRecipientsTextDiscovery creates discovery for dynamic alert recipients text entity.
func BuildRecipientsTextDiscovery(discoveryPrefix, topicPrefix string) (*DiscoveryMessage, error) {
	uniqueID := "gsm2mqtt_gateway_recipients"
	topic := fmt.Sprintf("%s/text/%s/config", discoveryPrefix, uniqueID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := TextDiscoveryPayload{
		Name:                "Alert Recipients",
		UniqueID:            uniqueID,
		ObjectID:            "gsm2mqtt_gateway_recipients",
		CommandTopic:        fmt.Sprintf("%s/config/recipients/set", topicPrefix),
		StateTopic:          fmt.Sprintf("%s/config/recipients", topicPrefix),
		CommandTemplate:     "{{ value }}",
		ValueTemplate:       "{{ value_json | join(', ') if value_json is iterable and value_json is not string else value }}",
		Icon:                "mdi:phone-message",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device: &DeviceInfo{
			Identifiers:  []string{GatewayIdentifier},
			Name:         "GSM2MQTT Gateway",
			Manufacturer: "GSM2MQTT",
			Model:        "Go GSM Gateway",
		},
	}
	return marshalDiscovery(topic, payload)
}

func gatewayDeviceInfo(ver string) *DeviceInfo {
	if ver == "" {
		ver = version.Version
	}
	return &DeviceInfo{
		Identifiers:  []string{GatewayIdentifier},
		Name:         "GSM2MQTT Gateway",
		Manufacturer: "GSM2MQTT",
		Model:        "Go GSM Gateway",
		SwVersion:    ver,
	}
}

// BuildGatewayDiscovery creates discovery message for the parent GSM2MQTT Gateway device.
func BuildGatewayDiscovery(discoveryPrefix, topicPrefix, version string) (*DiscoveryMessage, error) {
	uniqueID := "gsm2mqtt_gateway_status"
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Gateway Status",
		UniqueID:            uniqueID,
		ObjectID:            "gsm2mqtt_gateway_status",
		StateTopic:          availTopic,
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Icon:                "mdi:router-wireless",
		Device:              gatewayDeviceInfo(version),
	}
	return marshalDiscovery(topic, payload)
}

// BuildGatewayModemCountDiscovery creates discovery for connected modems count sensor.
func BuildGatewayModemCountDiscovery(discoveryPrefix, topicPrefix, version string) (*DiscoveryMessage, error) {
	uniqueID := "gsm2mqtt_gateway_modem_count"
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)
	stateTopic := fmt.Sprintf("%s/gateway/modems", topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Connected Modems",
		UniqueID:            uniqueID,
		ObjectID:            "gsm2mqtt_gateway_modem_count",
		StateTopic:          stateTopic,
		JSONAttributesTopic: stateTopic,
		ValueTemplate:       "{{ value_json.count }}",
		UnitOfMeasurement:   "modems",
		StateClass:          "measurement",
		Icon:                "mdi:devices",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              gatewayDeviceInfo(version),
	}
	return marshalDiscovery(topic, payload)
}

// BuildGatewayActiveModemDiscovery creates discovery for active modem name sensor.
func BuildGatewayActiveModemDiscovery(discoveryPrefix, topicPrefix, version string) (*DiscoveryMessage, error) {
	uniqueID := "gsm2mqtt_gateway_active_modem"
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)
	stateTopic := fmt.Sprintf("%s/gateway/modems", topicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Active Modem",
		UniqueID:            uniqueID,
		ObjectID:            "gsm2mqtt_gateway_active_modem",
		StateTopic:          stateTopic,
		JSONAttributesTopic: stateTopic,
		ValueTemplate:       "{{ value_json.active_modem }}",
		Icon:                "mdi:modem",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              gatewayDeviceInfo(version),
	}
	return marshalDiscovery(topic, payload)
}

func buildDeviceInfo(p ModemDiscoveryParams) *DeviceInfo {
	cleanMfg := strings.TrimSpace(p.Manufacturer)
	cleanModel := strings.TrimSpace(p.Model)
	if strings.EqualFold(cleanMfg, "undefined") || cleanMfg == "" {
		cleanMfg = "Unknown"
	}
	if cleanModel == "" {
		cleanModel = "Modem"
	}
	return &DeviceInfo{
		Identifiers:  []string{p.DeviceIdentifier()},
		Name:         p.DeviceName(),
		Manufacturer: cleanMfg,
		Model:        cleanModel,
		SwVersion:    p.SwVersion,
		ViaDevice:    GatewayIdentifier,
	}
}
