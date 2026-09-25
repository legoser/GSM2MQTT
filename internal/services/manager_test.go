package services

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/security"
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
			QoS:             2,
			CleanSession:    true,
			KeepAlive:       30 * time.Second,
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
		if st.QoS != 2 {
			t.Errorf("expected QoS 2, got %d", st.QoS)
		}
		if !st.CleanSession {
			t.Error("expected CleanSession true")
		}
		if st.KeepAliveSeconds != 30 {
			t.Errorf("expected KeepAliveSeconds 30, got %d", st.KeepAliveSeconds)
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

func TestGatewayManager_Recipients(t *testing.T) {
	mgr := NewGatewayManager()

	t.Run("uninitialized manager", func(t *testing.T) {
		if recs := mgr.GetRecipients(); recs != nil {
			t.Fatalf("expected nil recipients, got %v", recs)
		}
		if err := mgr.AddRecipient("+79991112233"); err == nil {
			t.Fatal("expected error adding recipient to uninitialized manager")
		}
		if ok := mgr.RemoveRecipient("+79991112233"); ok {
			t.Fatal("expected false removing from uninitialized manager")
		}
		if err := mgr.SetRecipients([]string{"+79991112233"}); err == nil {
			t.Fatal("expected error setting recipients on uninitialized manager")
		}
	})

	t.Run("initialized manager", func(t *testing.T) {
		tmpDir := t.TempDir()
		recFile := filepath.Join(tmpDir, "recipients.json")
		recMgr := security.NewRecipientsManager(recFile, []string{"+79991112233"})
		mgr.InitRecipients(recMgr)

		recs := mgr.GetRecipients()
		if len(recs) != 1 || recs[0] != "+79991112233" {
			t.Fatalf("expected [+79991112233], got %v", recs)
		}

		if err := mgr.AddRecipient("+79992223344"); err != nil {
			t.Fatalf("failed to add recipient: %v", err)
		}
		if len(mgr.GetRecipients()) != 2 {
			t.Fatalf("expected 2 recipients, got %d", len(mgr.GetRecipients()))
		}

		if !mgr.RemoveRecipient("+79991112233") {
			t.Fatal("expected successful removal")
		}
		if len(mgr.GetRecipients()) != 1 || mgr.GetRecipients()[0] != "+79992223344" {
			t.Fatalf("unexpected recipients after removal: %v", mgr.GetRecipients())
		}

		if err := mgr.SetRecipients([]string{"+79995556677", "+79998889900"}); err != nil {
			t.Fatalf("failed to set recipients: %v", err)
		}
		if len(mgr.GetRecipients()) != 2 {
			t.Fatalf("expected 2 recipients after set, got %d", len(mgr.GetRecipients()))
		}
	})
}

func TestGatewayManager_RecipientsMQTT(t *testing.T) {
	tmpDir := t.TempDir()
	recFile := filepath.Join(tmpDir, "recipients.json")
	recMgr := security.NewRecipientsManager(recFile, []string{"+79991112233"})

	mgr := NewGatewayManager()
	mgr.InitRecipients(recMgr)

	client := mqtt.NewMockClient()
	_ = client.Connect()

	cfg := &config.MQTTConfig{
		TopicPrefix: "gsm2mqtt",
	}

	var lastPublished []string
	_ = client.Subscribe("gsm2mqtt/config/recipients", 1, func(_ string, payload []byte) {
		var nums []string
		_ = json.Unmarshal(payload, &nums)
		lastPublished = nums
	})

	mgr.SetMQTT(client, cfg)

	if len(lastPublished) != 1 || lastPublished[0] != "+79991112233" {
		t.Fatalf("expected initial state publication, got %v", lastPublished)
	}

	t.Run("mqtt add recipient json", func(t *testing.T) {
		_ = client.Publish("gsm2mqtt/config/recipients/add", 1, false, []byte(`{"number":"+79992223344"}`))
		if len(mgr.GetRecipients()) != 2 {
			t.Fatalf("expected 2 recipients, got %v", mgr.GetRecipients())
		}
		if len(lastPublished) != 2 {
			t.Fatalf("expected published updated recipients, got %v", lastPublished)
		}
	})

	t.Run("mqtt add recipient plain text", func(t *testing.T) {
		_ = client.Publish("gsm2mqtt/config/recipients/add", 1, false, []byte(`+79993334455`))
		if len(mgr.GetRecipients()) != 3 {
			t.Fatalf("expected 3 recipients, got %v", mgr.GetRecipients())
		}
	})

	t.Run("mqtt remove recipient json", func(t *testing.T) {
		_ = client.Publish("gsm2mqtt/config/recipients/remove", 1, false, []byte(`{"number":"+79991112233"}`))
		if len(mgr.GetRecipients()) != 2 {
			t.Fatalf("expected 2 recipients, got %v", mgr.GetRecipients())
		}
	})

	t.Run("mqtt set recipients array", func(t *testing.T) {
		_ = client.Publish("gsm2mqtt/config/recipients/set", 1, false, []byte(`["+79997778899"]`))
		recs := mgr.GetRecipients()
		if len(recs) != 1 || recs[0] != "+79997778899" {
			t.Fatalf("expected [+79997778899], got %v", recs)
		}
	})

	t.Run("mqtt set recipients comma separated", func(t *testing.T) {
		_ = client.Publish("gsm2mqtt/config/recipients/set", 1, false, []byte(`+79991111111, +79992222222`))
		recs := mgr.GetRecipients()
		if len(recs) != 2 {
			t.Fatalf("expected 2 recipients, got %v", recs)
		}
	})
}
