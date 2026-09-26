package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/legoser/gsm2mqtt/internal/system"
)

const maxCallHistory = 100

// CallRecord represents a single recorded voice call (incoming or outgoing).
type CallRecord struct {
	ID        string `json:"id"`
	ModemID   string `json:"modem_id"`
	Number    string `json:"number"`
	Direction string `json:"direction"` // "incoming" or "outgoing"
	Status    string `json:"status"`    // "completed", "missed", "rejected", "failed", "busy", "no_answer"
	Duration  int    `json:"duration"`  // duration in seconds
	Timestamp string `json:"timestamp"` // formatted local time e.g. "2006-01-02 15:04:05"
}

func (r *ModemRunner) callHistoryFilePath() string {
	dir := "data"
	if r.cfg != nil && r.cfg.Tariff.StorageDir != "" {
		dir = r.cfg.Tariff.StorageDir
	}
	cleanID := safeModemIDRegex.ReplaceAllString(r.mCfg.ID, "_")
	if cleanID == "" {
		cleanID = "default"
	}
	return filepath.Join(dir, fmt.Sprintf("calls_%s.json", cleanID))
}

func (r *ModemRunner) loadCallHistory() {
	targetPath := r.callHistoryFilePath()
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return
	}
	var calls []CallRecord
	if err := json.Unmarshal(data, &calls); err == nil {
		r.mu.Lock()
		r.callHistory = calls
		r.mu.Unlock()
	}
}

func (r *ModemRunner) saveCallHistoryLocked() {
	dir := "data"
	if r.cfg != nil && r.cfg.Tariff.StorageDir != "" {
		dir = r.cfg.Tariff.StorageDir
	}
	_ = os.MkdirAll(dir, 0755)

	targetPath := r.callHistoryFilePath()
	tempPath := targetPath + ".tmp"

	data, err := json.MarshalIndent(r.callHistory, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tempPath, targetPath)
}

func (r *ModemRunner) recordCall(record CallRecord) {
	r.mu.Lock()
	if record.ID == "" {
		record.ID = fmt.Sprintf("%s-call-%d", r.mCfg.ID, time.Now().UnixNano())
	}
	if record.ModemID == "" {
		record.ModemID = r.mCfg.ID
	}
	if record.Timestamp == "" {
		record.Timestamp = system.FormatLocalTime(time.Now(), r.location())
	}
	r.callHistory = append(r.callHistory, record)
	if len(r.callHistory) > maxCallHistory {
		r.callHistory = r.callHistory[len(r.callHistory)-maxCallHistory:]
	}
	r.saveCallHistoryLocked()
	r.mu.Unlock()

	r.publishCallHistory()
}

// GetCallHistory returns a copy of recorded calls for this modem runner.
func (r *ModemRunner) GetCallHistory() []CallRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]CallRecord, len(r.callHistory))
	copy(res, r.callHistory)
	return res
}

// ClearCallHistory wipes recorded calls for this modem runner and updates MQTT.
func (r *ModemRunner) ClearCallHistory() {
	r.mu.Lock()
	r.callHistory = []CallRecord{}
	r.saveCallHistoryLocked()
	r.mu.Unlock()

	r.publishCallHistory()
}

// publishCallHistory publishes the call history payload to MQTT (retained).
func (r *ModemRunner) publishCallHistory() {
	if r.mqttClient == nil || !r.mqttClient.IsConnected() {
		return
	}
	r.mu.RLock()
	calls := make([]CallRecord, len(r.callHistory))
	copy(calls, r.callHistory)
	r.mu.RUnlock()

	payload, err := json.Marshal(map[string]any{
		"count": len(calls),
		"items": calls,
	})
	if err == nil {
		_ = r.mqttClient.Publish(r.topics.CallHistory(), r.qos(), true, payload)
	}
}
