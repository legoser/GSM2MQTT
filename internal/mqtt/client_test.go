package mqtt

import (
	"errors"
	"testing"
)

func TestClientConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientConfig
		wantErr bool
	}{
		{
			name: "valid broker",
			cfg: ClientConfig{
				Broker: "tcp://localhost:1883",
			},
			wantErr: false,
		},
		{
			name: "empty broker",
			cfg: ClientConfig{
				Broker: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestMockClient_PubSub(t *testing.T) {
	mock := NewMockClient()

	if err := mock.Connect(); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	var receivedTopic string
	var receivedPayload []byte

	err := mock.Subscribe("gsm2mqtt/test", 1, func(topic string, payload []byte) {
		receivedTopic = topic
		receivedPayload = payload
	})
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	msg := []byte("hello mqtt")
	if err := mock.Publish("gsm2mqtt/test", 1, false, msg); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	if receivedTopic != "gsm2mqtt/test" || string(receivedPayload) != "hello mqtt" {
		t.Errorf("expected received msg 'hello mqtt', got %q", string(receivedPayload))
	}

	mock.Disconnect(0)
	if mock.IsConnected() {
		t.Errorf("expected IsConnected to be false after disconnect")
	}

	if err := mock.Publish("gsm2mqtt/test", 1, false, msg); !errors.Is(err, ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}
