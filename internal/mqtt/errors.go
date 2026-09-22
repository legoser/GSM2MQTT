package mqtt

import "errors"

var (
	// ErrNotConnected is returned when an MQTT operation is attempted while disconnected.
	ErrNotConnected = errors.New("mqtt client is not connected")

	// ErrEmptyTopic is returned when an empty MQTT topic is specified.
	ErrEmptyTopic = errors.New("mqtt topic cannot be empty")
)
