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
