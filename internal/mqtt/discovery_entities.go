package mqtt

import (
	"encoding/json"
	"fmt"
)

// BuildCheckBalanceButtonDiscovery generates discovery for USSD Check Balance button.
func BuildCheckBalanceButtonDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_btn_balance", modemID)
	topic := fmt.Sprintf("%s/button/%s/config", discoveryPrefix, uniqueID)
	cmdTopic := fmt.Sprintf("%s/modem/%s/ussd/send", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := ButtonDiscoveryPayload{
		Name:                "Check Balance",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_check_balance", modemID),
		CommandTopic:        cmdTopic,
		PayloadPress:        "*100#",
		Icon:                "mdi:cash-sync",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildHangupButtonDiscovery generates discovery for Voice Call Hangup button.
func BuildHangupButtonDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_btn_hangup", modemID)
	topic := fmt.Sprintf("%s/button/%s/config", discoveryPrefix, uniqueID)
	cmdTopic := fmt.Sprintf("%s/modem/%s/call/hangup", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := ButtonDiscoveryPayload{
		Name:                "Hangup Call",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_hangup", modemID),
		CommandTopic:        cmdTopic,
		PayloadPress:        "{}",
		Icon:                "mdi:phone-hangup",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildIncomingCallDiscovery generates discovery for incoming voice call binary sensor.
func BuildIncomingCallDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_incoming_call", modemID)
	topic := fmt.Sprintf("%s/binary_sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/call/incoming", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := BinarySensorDiscoveryPayload{
		Name:                "Incoming Call",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_incoming_call", modemID),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ value_json.type }}",
		PayloadOn:           "incoming",
		OffDelay:            30,
		Icon:                "mdi:phone-ring",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildNewSMSBinaryDiscovery generates discovery for new SMS received pulse binary sensor.
func BuildNewSMSBinaryDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model string) (*DiscoveryMessage, error) {
	uniqueID := fmt.Sprintf("gsm2mqtt_%s_new_sms", modemID)
	topic := fmt.Sprintf("%s/binary_sensor/%s/config", discoveryPrefix, uniqueID)
	stateTopic := fmt.Sprintf("%s/modem/%s/sms/received", topicPrefix, modemID)
	availTopic, avail, notAvail := buildAvailability(topicPrefix)

	payload := BinarySensorDiscoveryPayload{
		Name:                "New SMS Received",
		UniqueID:            uniqueID,
		ObjectID:            fmt.Sprintf("%s_new_sms", modemID),
		StateTopic:          stateTopic,
		ValueTemplate:       "{{ 'ON' if value_json.text is defined else 'OFF' }}",
		PayloadOn:           "ON",
		OffDelay:            5,
		Icon:                "mdi:message-badge",
		AvailabilityTopic:   availTopic,
		PayloadAvailable:    avail,
		PayloadNotAvailable: notAvail,
		Device:              buildDeviceInfo(modemID, manufacturer, model),
	}

	return marshalDiscovery(topic, payload)
}

// BuildModemDiscoveries builds the complete set of Home Assistant Auto-Discovery messages for a modem.
func BuildModemDiscoveries(discoveryPrefix, topicPrefix, modemID, manufacturer, model, currency string) ([]*DiscoveryMessage, error) {
	builders := []func() (*DiscoveryMessage, error){
		func() (*DiscoveryMessage, error) {
			return BuildSignalDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildBalanceDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model, currency)
		},
		func() (*DiscoveryMessage, error) {
			return BuildStatusDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildOperatorDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildLastSMSDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildUSSDResponseDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildCheckBalanceButtonDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildHangupButtonDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildIncomingCallDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
		},
		func() (*DiscoveryMessage, error) {
			return BuildNewSMSBinaryDiscovery(discoveryPrefix, topicPrefix, modemID, manufacturer, model)
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
