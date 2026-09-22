package tariff

import (
	"fmt"
	"sync"
	"time"
)

// Config defines accounting parameters, quotas, and alerting thresholds.
type Config struct {
	Enabled          bool          `yaml:"enabled"`
	OperatorPreset   string        `yaml:"operator_preset"`
	BalanceUSSD      string        `yaml:"balance_ussd"`
	BalanceRegex     string        `yaml:"balance_regex"`
	AutoCheckOnError bool          `yaml:"auto_check_on_error"`
	CheckInterval    time.Duration `yaml:"check_interval"`
	MinBalanceAlert  float64       `yaml:"min_balance_alert"`
	SMSLimit         int           `yaml:"sms_limit"`
	ResetDayOfMonth  int           `yaml:"reset_day_of_month"`
}

// UsageStatus represents current accounting snapshot for a modem.
type UsageStatus struct {
	Balance          float64   `json:"balance"`
	Currency         string    `json:"currency"`
	SMSDayCount      int       `json:"sms_day_count"`
	SMSMonthCount    int       `json:"sms_month_count"`
	SMSLimit         int       `json:"sms_limit"`
	SMSRemaining     int       `json:"sms_remaining"`
	DataBytesUsed    int64     `json:"data_bytes_used"`
	LastBalanceCheck time.Time `json:"last_balance_check,omitempty"`
	ResetDayOfMonth  int       `json:"reset_day_of_month"`
}

// AlertEvent represents a tariff alert emitted when quotas or balance limits are breached.
type AlertEvent struct {
	Type    string  `json:"type"` // low_balance, sms_limit_warning, sms_limit_exceeded
	Message string  `json:"message"`
	Value   float64 `json:"value"`
	ModemID string  `json:"modem_id"`
}

// Manager tracks SMS quotas, mobile data usage, and balance thresholds.
type Manager struct {
	mu               sync.RWMutex
	modemID          string
	cfg              Config
	onAlert          func(alert AlertEvent)
	balance          float64
	currency         string
	smsDayCount      int
	smsMonthCount    int
	dataBytesUsed    int64
	lastBalanceCheck time.Time
	lastDailyCheck   time.Time
	lastMonthlyCheck time.Time
	warned90         bool
	warnedExceeded   bool
	warnedLowBalance bool
}

// NewManager constructs a new tariff Manager.
func NewManager(modemID string, cfg Config, onAlert func(alert AlertEvent)) *Manager {
	if cfg.ResetDayOfMonth <= 0 || cfg.ResetDayOfMonth > 31 {
		cfg.ResetDayOfMonth = 1
	}
	return &Manager{
		modemID:          modemID,
		cfg:              cfg,
		onAlert:          onAlert,
		currency:         "RUB",
		lastDailyCheck:   time.Now(),
		lastMonthlyCheck: time.Now(),
	}
}

// RecordSMS increments daily and monthly message counters and evaluates quotas.
func (m *Manager) RecordSMS(count int) {
	if count <= 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.smsDayCount += count
	m.smsMonthCount += count

	m.evaluateSMSLimits()
}

// RecordData records transmitted data bytes.
func (m *Manager) RecordData(bytes int64) {
	if bytes <= 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataBytesUsed += bytes
}

// UpdateBalance updates the known monetary balance and checks thresholds.
func (m *Manager) UpdateBalance(balance float64, currency string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.balance = balance
	if currency != "" {
		m.currency = currency
	}
	m.lastBalanceCheck = time.Now()

	m.evaluateBalanceLimits(balance)
}

// ResetDaily zeroes the daily counter.
func (m *Manager) ResetDaily() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.smsDayCount = 0
}

// ResetMonthly zeroes the monthly counter and resets warning flags.
func (m *Manager) ResetMonthly() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.smsMonthCount = 0
	m.warned90 = false
	m.warnedExceeded = false
}

// Status returns a copy of current usage statistics.
func (m *Manager) Status() UsageStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	remaining := 0
	if m.cfg.SMSLimit > 0 {
		remaining = m.cfg.SMSLimit - m.smsMonthCount
		if remaining < 0 {
			remaining = 0
		}
	}

	return UsageStatus{
		Balance:          m.balance,
		Currency:         m.currency,
		SMSDayCount:      m.smsDayCount,
		SMSMonthCount:    m.smsMonthCount,
		SMSLimit:         m.cfg.SMSLimit,
		SMSRemaining:     remaining,
		DataBytesUsed:    m.dataBytesUsed,
		LastBalanceCheck: m.lastBalanceCheck,
		ResetDayOfMonth:  m.cfg.ResetDayOfMonth,
	}
}

func (m *Manager) evaluateSMSLimits() {
	if m.cfg.SMSLimit <= 0 || m.onAlert == nil {
		return
	}

	ratio := float64(m.smsMonthCount) / float64(m.cfg.SMSLimit)

	if ratio >= 0.9 && ratio < 1.0 && !m.warned90 {
		m.warned90 = true
		m.onAlert(AlertEvent{
			Type:    "sms_limit_warning",
			Message: fmt.Sprintf("90%% of monthly SMS quota reached (%d/%d)", m.smsMonthCount, m.cfg.SMSLimit),
			Value:   float64(m.smsMonthCount),
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedExceeded {
		m.warnedExceeded = true
		m.onAlert(AlertEvent{
			Type:    "sms_limit_exceeded",
			Message: fmt.Sprintf("Monthly SMS quota exceeded (%d/%d)", m.smsMonthCount, m.cfg.SMSLimit),
			Value:   float64(m.smsMonthCount),
			ModemID: m.modemID,
		})
	}
}

func (m *Manager) evaluateBalanceLimits(balance float64) {
	if m.cfg.MinBalanceAlert <= 0 || m.onAlert == nil {
		return
	}

	if balance < m.cfg.MinBalanceAlert {
		if !m.warnedLowBalance {
			m.warnedLowBalance = true
			m.onAlert(AlertEvent{
				Type:    "low_balance",
				Message: fmt.Sprintf("Balance is low: %.2f %s (threshold: %.2f)", balance, m.currency, m.cfg.MinBalanceAlert),
				Value:   balance,
				ModemID: m.modemID,
			})
		}
	} else {
		m.warnedLowBalance = false
	}
}
