package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/security"
)

// InitRecipients registers the dynamic alert recipients collection and hooks up change publication.
func (m *GatewayManager) InitRecipients(mgr *security.RecipientsManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recipientsMgr = mgr
	if m.recipientsMgr != nil {
		m.recipientsMgr.SetOnChange(func(nums []string) {
			m.publishRecipientsState(nums)
		})
	}
}

// GetRecipients returns all registered alert recipient numbers.
func (m *GatewayManager) GetRecipients() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.recipientsMgr != nil {
		return m.recipientsMgr.Get()
	}
	return nil
}

// AddRecipient adds an alert recipient number.
func (m *GatewayManager) AddRecipient(raw string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.recipientsMgr != nil {
		return m.recipientsMgr.Add(raw)
	}
	return fmt.Errorf("recipients manager not initialized")
}

// RemoveRecipient removes an alert recipient number.
func (m *GatewayManager) RemoveRecipient(raw string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.recipientsMgr != nil {
		return m.recipientsMgr.Remove(raw)
	}
	return false
}

// SetRecipients replaces the full list of alert recipient numbers.
func (m *GatewayManager) SetRecipients(numbers []string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.recipientsMgr != nil {
		return m.recipientsMgr.Set(numbers)
	}
	return fmt.Errorf("recipients manager not initialized")
}

func (m *GatewayManager) publishRecipientsState(nums []string) {
	m.mu.RLock()
	client := m.mqttClient
	prefix := m.mqttCfg.TopicPrefix
	m.mu.RUnlock()

	if client != nil && client.IsConnected() && prefix != "" {
		payload, _ := json.Marshal(nums)
		_ = client.Publish(fmt.Sprintf("%s/config/recipients", prefix), 1, true, payload)
	}
}

func (m *GatewayManager) subscribeRecipientsMQTT(client mqtt.MQTTClient, prefix string) {
	_ = client.Subscribe(fmt.Sprintf("%s/config/recipients/set", prefix), 1, func(_ string, payload []byte) {
		var nums []string
		if err := json.Unmarshal(payload, &nums); err != nil {
			raw := string(payload)
			for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
				return r == ',' || r == '\n' || r == ';'
			}) {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					nums = append(nums, trimmed)
				}
			}
		}
		slog.Info("received mqtt request to set alert recipients list", slog.Int("count", len(nums)))
		if err := m.SetRecipients(nums); err != nil {
			slog.Error("failed to set alert recipients from mqtt", slog.Any("error", err))
		}
	})

	_ = client.Subscribe(fmt.Sprintf("%s/config/recipients/add", prefix), 1, func(_ string, payload []byte) {
		var req struct {
			Number string `json:"number"`
		}
		raw := strings.TrimSpace(string(payload))
		if err := json.Unmarshal(payload, &req); err == nil && req.Number != "" {
			raw = req.Number
		}
		raw = strings.Trim(raw, "\"")
		if raw != "" {
			slog.Info("received mqtt request to add alert recipient", slog.String("raw", raw))
			if err := m.AddRecipient(raw); err != nil {
				slog.Error("failed to add alert recipient from mqtt", slog.String("raw", raw), slog.Any("error", err))
			}
		}
	})

	_ = client.Subscribe(fmt.Sprintf("%s/config/recipients/remove", prefix), 1, func(_ string, payload []byte) {
		var req struct {
			Number string `json:"number"`
		}
		raw := strings.TrimSpace(string(payload))
		if err := json.Unmarshal(payload, &req); err == nil && req.Number != "" {
			raw = req.Number
		}
		raw = strings.Trim(raw, "\"")
		if raw != "" {
			slog.Info("received mqtt request to remove alert recipient", slog.String("raw", raw))
			if ok := m.RemoveRecipient(raw); !ok {
				slog.Warn("failed to remove alert recipient from mqtt", slog.String("raw", raw))
			}
		}
	})
}
