package services

import (
	"testing"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
)

func TestGatewayManager_MQTTStatus(t *testing.T) {
	mgr := NewGatewayManager()

	t.Run("nil mqtt client", func(t *testing.T) {
		st := mgr.GetMQTTStatus()
		if st.Connected {
			t.Error("expected connected false when client is nil")
		}
	})

	t.Run("connected mqtt client with config", func(t *testing.T) {
		client := mqtt.NewMockClient()
		_ = client.Connect()

		cfg := &config.MQTTConfig{
			Broker:          "tcp://127.0.0.1",
			Port:            1883,
			ClientID:        "test-gsm2mqtt",
			TopicPrefix:     "gsm2mqtt",
			Username:        "admin",
			Password:        "secret123",
			Discovery:       true,
			DiscoveryPrefix: "homeassistant",
		}

		mgr.SetMQTT(client, cfg)
		st := mgr.GetMQTTStatus()

		if !st.Connected {
			t.Error("expected connected true")
		}
		if st.Broker != "tcp://127.0.0.1" {
			t.Errorf("expected broker tcp://127.0.0.1, got %s", st.Broker)
		}
		if st.ClientID != "test-gsm2mqtt" {
			t.Errorf("expected client_id test-gsm2mqtt, got %s", st.ClientID)
		}
		if st.TopicPrefix != "gsm2mqtt" {
			t.Errorf("expected topic_prefix gsm2mqtt, got %s", st.TopicPrefix)
		}
		if st.Username != "admin" {
			t.Errorf("expected username admin, got %s", st.Username)
		}
		if !st.Discovery {
			t.Error("expected discovery true")
		}
	})

	t.Run("disconnected client", func(t *testing.T) {
		client := mqtt.NewMockClient()
		mgr.SetMQTT(client, nil)
		st := mgr.GetMQTTStatus()
		if st.Connected {
			t.Error("expected connected false for disconnected client")
		}
	})
}
