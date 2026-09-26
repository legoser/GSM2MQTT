package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/legoser/gsm2mqtt/internal/sms"
)

const maxInboxMessages = 100

var safeModemIDRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func (r *ModemRunner) inboxFilePath() string {
	dir := "data"
	if r.cfg != nil && r.cfg.Tariff.StorageDir != "" {
		dir = r.cfg.Tariff.StorageDir
	}
	cleanID := safeModemIDRegex.ReplaceAllString(r.mCfg.ID, "_")
	if cleanID == "" {
		cleanID = "default"
	}
	return filepath.Join(dir, fmt.Sprintf("inbox_%s.json", cleanID))
}

func (r *ModemRunner) loadInbox() {
	targetPath := r.inboxFilePath()
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return
	}
	var msgs []ReceivedSMS
	if err := json.Unmarshal(data, &msgs); err == nil {
		r.mu.Lock()
		r.receivedSMS = msgs
		r.mu.Unlock()
	}
}

func (r *ModemRunner) saveInboxLocked() {
	dir := "data"
	if r.cfg != nil && r.cfg.Tariff.StorageDir != "" {
		dir = r.cfg.Tariff.StorageDir
	}
	_ = os.MkdirAll(dir, 0755)

	targetPath := r.inboxFilePath()
	tempPath := targetPath + ".tmp"

	data, err := json.MarshalIndent(r.receivedSMS, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tempPath, targetPath)
}

func (r *ModemRunner) recordIncomingSMS(msg *sms.AssembledSMS) {
	r.mu.Lock()

	tsStr := msg.Timestamp.Format("2006-01-02 15:04:05")

	// Deduplication: check if already recorded
	for _, existing := range r.receivedSMS {
		if existing.Sender == msg.From && existing.Text == msg.Text && existing.Timestamp == tsStr {
			r.mu.Unlock()
			return
		}
	}

	entry := ReceivedSMS{
		ID:        fmt.Sprintf("%s-%d", r.mCfg.ID, time.Now().UnixNano()),
		ModemID:   r.mCfg.ID,
		Sender:    msg.From,
		From:      msg.From,
		Timestamp: tsStr,
		Text:      msg.Text,
	}
	r.receivedSMS = append(r.receivedSMS, entry)
	if len(r.receivedSMS) > maxInboxMessages {
		r.receivedSMS = r.receivedSMS[len(r.receivedSMS)-maxInboxMessages:]
	}
	r.saveInboxLocked()
	r.mu.Unlock()

	r.publishSMSHistory()
}

// GetReceivedSMS returns the list of recently received SMS messages for this modem.
func (r *ModemRunner) GetReceivedSMS() []ReceivedSMS {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]ReceivedSMS, len(r.receivedSMS))
	copy(res, r.receivedSMS)
	return res
}

// ClearReceivedSMS wipes the SMS history for this modem runner and updates MQTT.
func (r *ModemRunner) ClearReceivedSMS() {
	r.mu.Lock()
	r.receivedSMS = []ReceivedSMS{}
	r.saveInboxLocked()
	r.mu.Unlock()

	r.publishSMSHistory()
	if r.mqttClient != nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(r.topics.SMSLast(), r.qos(), true, []byte(`{"text":"","from":"","timestamp":""}`))
	}
}

// publishSMSHistory publishes the current list of received SMS to MQTT (retained).
func (r *ModemRunner) publishSMSHistory() {
	if r.mqttClient == nil || !r.mqttClient.IsConnected() {
		return
	}
	r.mu.RLock()
	msgs := make([]ReceivedSMS, len(r.receivedSMS))
	copy(msgs, r.receivedSMS)
	r.mu.RUnlock()

	payload, err := json.Marshal(map[string]any{
		"count": len(msgs),
		"items": msgs,
	})
	if err == nil {
		_ = r.mqttClient.Publish(r.topics.SMSHistory(), r.qos(), true, payload)
	}
}
