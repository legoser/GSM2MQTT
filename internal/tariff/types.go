package tariff

import "time"

// Config defines accounting parameters, quotas, and alerting thresholds.
type Config struct {
	Enabled            bool          `yaml:"enabled"`
	OperatorPreset     string        `yaml:"operator_preset"`
	BalanceUSSD        string        `yaml:"balance_ussd"`
	BalanceRegex       string        `yaml:"balance_regex"`
	AutoCheckOnError   bool          `yaml:"auto_check_on_error"`
	CheckInterval      time.Duration `yaml:"check_interval"`
	MinBalanceAlert    float64       `yaml:"min_balance_alert"`
	SMSLimit           int           `yaml:"sms_limit"`
	CallMinutesLimit   float64       `yaml:"call_minutes_limit"`
	DataTrafficLimitMB int64         `yaml:"data_traffic_limit_mb"`
	ResetDayOfMonth    int           `yaml:"reset_day_of_month"`
	StorageDir         string        `yaml:"storage_dir"`
}

// UsageStatus represents current accounting snapshot for a modem.
type UsageStatus struct {
	Balance              float64   `json:"balance"`
	Currency             string    `json:"currency"`
	SMSDayCount          int       `json:"sms_day_count"`
	SMSMonthCount        int       `json:"sms_month_count"`
	SMSLimit             int       `json:"sms_limit"`
	SMSRemaining         int       `json:"sms_remaining"`
	CallMinutesLimit     float64   `json:"call_minutes_limit"`
	CallMinutesUsed      float64   `json:"call_minutes_used"`
	CallMinutesRemaining float64   `json:"call_minutes_remaining"`
	DataTrafficLimitMB   int64     `json:"data_traffic_limit_mb"`
	DataBytesLimit       int64     `json:"data_bytes_limit"`
	DataBytesUsed        int64     `json:"data_bytes_used"`
	DataBytesRemaining   int64     `json:"data_bytes_remaining"`
	LastBalanceCheck     time.Time `json:"last_balance_check,omitempty"`
	ResetDayOfMonth      int       `json:"reset_day_of_month"`
}

// AlertEvent represents a tariff alert emitted when quotas or balance limits are breached.
type AlertEvent struct {
	Type    string  `json:"type"` // low_balance, sms_limit_warning, sms_limit_exceeded, etc.
	Message string  `json:"message"`
	Value   float64 `json:"value"`
	ModemID string  `json:"modem_id"`
}
