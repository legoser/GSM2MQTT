package mqtt

import (
	"fmt"
)

// BuildTariffSMSDiscovery generates discovery for SMS remaining quota sensor.
func BuildTariffSMSDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("sms_remaining")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/accounting/status", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "SMS Remaining",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("sms_remaining"),
		StateTopic:          stateTopic,
		JSONAttributesTopic: stateTopic,
		ValueTemplate:       "{{ value_json.sms_remaining }}",
		UnitOfMeasurement:   "SMS",
		StateClass:          "measurement",
		Icon:                "mdi:message-badge-outline",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildTariffCallMinutesDiscovery generates discovery for Call minutes remaining quota sensor.
func BuildTariffCallMinutesDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("call_minutes_remaining")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/accounting/status", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Call Minutes Remaining",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("call_minutes_remaining"),
		StateTopic:          stateTopic,
		JSONAttributesTopic: stateTopic,
		ValueTemplate:       "{{ value_json.call_minutes_remaining }}",
		UnitOfMeasurement:   "min",
		StateClass:          "measurement",
		Icon:                "mdi:phone-clock",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildTariffResetButtonDiscovery generates discovery for Reset Tariff Quotas button.
func BuildTariffResetButtonDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("btn_tariff_reset")
	topic := fmt.Sprintf("%s/button/%s/config", p.DiscoveryPrefix, uniqueID)
	cmdTopic := fmt.Sprintf("%s/modem/%s/tariff/reset", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := ButtonDiscoveryPayload{
		Name:                "Reset Tariff Quotas",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("tariff_reset"),
		CommandTopic:        cmdTopic,
		PayloadPress:        "{}",
		Icon:                "mdi:restore",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildTariffDataTrafficDiscovery generates discovery for mobile data traffic remaining sensor.
func BuildTariffDataTrafficDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("data_traffic_remaining")
	topic := fmt.Sprintf("%s/sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/accounting/status", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := SensorDiscoveryPayload{
		Name:                "Data Traffic Remaining",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("data_traffic_remaining"),
		StateTopic:          stateTopic,
		JSONAttributesTopic: stateTopic,
		ValueTemplate:       "{{ (value_json.data_bytes_remaining / 1048576) | round(1) }}",
		UnitOfMeasurement:   "MB",
		StateClass:          "measurement",
		Icon:                "mdi:web",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildTariffLowBalanceBinaryDiscovery generates discovery for low balance warning binary sensor.
func BuildTariffLowBalanceBinaryDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("low_balance")
	topic := fmt.Sprintf("%s/binary_sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/accounting/status", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := BinarySensorDiscoveryPayload{
		Name:                "Low Balance Warning",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("low_balance"),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ 'ON' if value_json.low_balance else 'OFF' }}",
		PayloadOn:           "ON",
		PayloadOff:          "OFF",
		DeviceClass:         "problem",
		Icon:                "mdi:cash-alert",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}
