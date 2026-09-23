package mqtt

import (
	"fmt"
	"strings"
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

// BuildGatewayDiscovery creates discovery message for the parent GSM2MQTT Gateway device.
func BuildGatewayDiscovery(discoveryPrefix, topicPrefix, version string) (*DiscoveryMessage, error) {
	uniqueID := "gsm2mqtt_gateway_status"
	topic := fmt.Sprintf("%s/sensor/%s/config", discoveryPrefix, uniqueID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	if version == "" {
		version = "1.0.0"
	}

	payload := SensorDiscoveryPayload{
		Name:                "Gateway Status",
		UniqueID:            uniqueID,
		ObjectID:            "gsm2mqtt_gateway_status",
		StateTopic:          availTopic,
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Icon:                "mdi:router-wireless",
		Device: &DeviceInfo{
			Identifiers:  []string{GatewayIdentifier},
			Name:         "GSM2MQTT Gateway",
			Manufacturer: "GSM2MQTT",
			Model:        "Go GSM Gateway",
			SwVersion:    version,
		},
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
