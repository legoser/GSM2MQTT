package mqtt

import (
	"encoding/json"
	"fmt"
)

// BuildCheckBalanceButtonDiscovery generates discovery for USSD Check Balance button.
func BuildCheckBalanceButtonDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("btn_balance")
	topic := fmt.Sprintf("%s/button/%s/config", p.DiscoveryPrefix, uniqueID)
	cmdTopic := fmt.Sprintf("%s/modem/%s/ussd/send", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := ButtonDiscoveryPayload{
		Name:                "Check Balance",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("check_balance"),
		CommandTopic:        cmdTopic,
		PayloadPress:        "*100#",
		Icon:                "mdi:cash-sync",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildHangupButtonDiscovery generates discovery for Voice Call Hangup button.
func BuildHangupButtonDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("btn_hangup")
	topic := fmt.Sprintf("%s/button/%s/config", p.DiscoveryPrefix, uniqueID)
	cmdTopic := fmt.Sprintf("%s/modem/%s/call/hangup", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := ButtonDiscoveryPayload{
		Name:                "Hangup Call",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("hangup"),
		CommandTopic:        cmdTopic,
		PayloadPress:        "{}",
		Icon:                "mdi:phone-hangup",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildIncomingCallDiscovery generates discovery for incoming voice call binary sensor.
func BuildIncomingCallDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("incoming_call")
	topic := fmt.Sprintf("%s/binary_sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/call/incoming", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := BinarySensorDiscoveryPayload{
		Name:                "Incoming Call",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("incoming_call"),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.type }}",
		PayloadOn:           "incoming",
		OffDelay:            30,
		Icon:                "mdi:phone-ring",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildNewSMSBinaryDiscovery generates discovery for new SMS received pulse binary sensor.
func BuildNewSMSBinaryDiscovery(p ModemDiscoveryParams) (*DiscoveryMessage, error) {
	uniqueID := p.EntityUniqueID("new_sms")
	topic := fmt.Sprintf("%s/binary_sensor/%s/config", p.DiscoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/sms/received", p.TopicPrefix, p.ModemID)
	availTopic, avail, notAvail := buildAvailability(p.TopicPrefix)

	payload := BinarySensorDiscoveryPayload{
		Name:                "New SMS Received",
		UniqueID:            uniqueID,
		ObjectID:            p.EntityObjectID("new_sms"),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ 'ON' if value_json.text is defined else 'OFF' }}",
		PayloadOn:           "ON",
		OffDelay:            5,
		Icon:                "mdi:message-badge",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(p),
	}

	return marshalDiscovery(topic, payload)
}

// BuildModemDiscoveries builds the complete set of Home Assistant Auto-Discovery messages for a modem.
func BuildModemDiscoveries(p ModemDiscoveryParams) ([]*DiscoveryMessage, error) {
	builders := []func() (*DiscoveryMessage, error){
		func() (*DiscoveryMessage, error) {
			return BuildSignalDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildBalanceDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildStatusDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildOperatorDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildLastSMSDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildUSSDResponseDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildCheckBalanceButtonDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildHangupButtonDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildIncomingCallDiscovery(p)
		},
		func() (*DiscoveryMessage, error) {
			return BuildNewSMSBinaryDiscovery(p)
		},
	}

	var messages []*DiscoveryMessage
	for _, b := range builders {
		msg, err := b()
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func marshalDiscovery(topic string, payload any) (*DiscoveryMessage, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery for %s: %w", topic, err)
	}
	return &DiscoveryMessage{
		Topic:   topic,
		Payload: data,
	}, nil
}
