package tariff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// DefaultCurrency defines standard currency when none is specified.
const DefaultCurrency = "RUB"

// State represents persisted quota usage and balance for a modem.
type State struct {
	Balance               float64   `json:"balance"`
	Currency              string    `json:"currency"`
	SMSDayCount           int       `json:"sms_day_count"`
	SMSMonthCount         int       `json:"sms_month_count"`
	CallMinutesUsed       float64   `json:"call_minutes_used"`
	DataBytesUsed         int64     `json:"data_bytes_used"`
	LastBalanceCheck      time.Time `json:"last_balance_check,omitempty"`
	LastDailyResetDate    string    `json:"last_daily_reset_date,omitempty"`
	LastMonthlyResetMonth string    `json:"last_monthly_reset_month,omitempty"`
	SMSLimit              int       `json:"sms_limit,omitempty"`
	CallMinutesLimit      float64   `json:"call_minutes_limit,omitempty"`
	DataTrafficLimitMB    int64     `json:"data_traffic_limit_mb,omitempty"`
	ResetDayOfMonth       int       `json:"reset_day_of_month,omitempty"`
	MinBalanceAlert       float64   `json:"min_balance_alert,omitempty"`
	BalanceUSSD           string    `json:"balance_ussd,omitempty"`
	OperatorPreset        string    `json:"operator_preset,omitempty"`
}

// Store persists and loads tariff state.
type Store interface {
	Save(modemID string, state State) error
	Load(modemID string) (*State, error)
}

// FileStore implements disk-backed JSON persistence for tariff states.
type FileStore struct {
	dir string
}

// NewFileStore creates a new FileStore in the given directory.
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

var safeFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func (s *FileStore) filePath(modemID string) string {
	cleanID := safeFilenameRegex.ReplaceAllString(modemID, "_")
	if cleanID == "" {
		cleanID = "default"
	}
	return filepath.Join(s.dir, fmt.Sprintf("tariff_%s.json", cleanID))
}

// Save writes modem tariff state to disk atomically.
func (s *FileStore) Save(modemID string, state State) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create tariff store directory: %w", err)
	}

	targetPath := s.filePath(modemID)
	tempPath := targetPath + ".tmp"

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tariff state: %w", err)
	}

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary tariff state: %w", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to atomically rename tariff state file: %w", err)
	}

	return nil
}

// Load reads modem tariff state from disk. Returns nil, nil if the file does not exist.
func (s *FileStore) Load(modemID string) (*State, error) {
	targetPath := s.filePath(modemID)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read tariff state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to decode tariff state JSON: %w", err)
	}

	return &state, nil
}
