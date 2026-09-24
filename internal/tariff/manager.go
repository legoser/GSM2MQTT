package tariff

import (
	"log/slog"
	"sync"
	"time"
)

// Manager tracks SMS quotas, mobile data usage, and balance thresholds.
type Manager struct {
	mu                    sync.RWMutex
	modemID               string
	cfg                   Config
	store                 Store
	onAlert               func(alert AlertEvent)
	balance               float64
	currency              string
	smsDayCount           int
	smsMonthCount         int
	callMinutesUsed       float64
	dataBytesUsed         int64
	lastBalanceCheck      time.Time
	lastDailyCheck        time.Time
	lastMonthlyCheck      time.Time
	lastDailyResetDate    string
	lastMonthlyResetMonth string
	warnedSMS90           bool
	warnedSMSExceeded     bool
	warnedCall90          bool
	warnedCallExceeded    bool
	warnedData90          bool
	warnedDataExceeded    bool
	warnedLowBalance      bool
}

// NewManager constructs a new tariff Manager.
func NewManager(modemID string, cfg Config, onAlert func(alert AlertEvent)) *Manager {
	if cfg.ResetDayOfMonth <= 0 || cfg.ResetDayOfMonth > 31 {
		cfg.ResetDayOfMonth = 1
	}
	m := &Manager{
		modemID:          modemID,
		cfg:              cfg,
		onAlert:          onAlert,
		currency:         DefaultCurrency,
		lastDailyCheck:   time.Now(),
		lastMonthlyCheck: time.Now(),
	}
	if cfg.StorageDir != "" {
		m.SetStore(NewFileStore(cfg.StorageDir))
	}
	return m
}

// SetStore assigns a persistence store and restores state if available.
func (m *Manager) SetStore(store Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
	m.restoreFromStore()
}

// UpdateConfig dynamically modifies tariff limits and parameters, persisting them.
func (m *Manager) UpdateConfig(newCfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if newCfg.SMSLimit >= 0 {
		m.cfg.SMSLimit = newCfg.SMSLimit
	}
	if newCfg.CallMinutesLimit >= 0 {
		m.cfg.CallMinutesLimit = newCfg.CallMinutesLimit
	}
	if newCfg.DataTrafficLimitMB >= 0 {
		m.cfg.DataTrafficLimitMB = newCfg.DataTrafficLimitMB
	}
	if newCfg.ResetDayOfMonth >= 1 && newCfg.ResetDayOfMonth <= 31 {
		m.cfg.ResetDayOfMonth = newCfg.ResetDayOfMonth
	}
	if newCfg.MinBalanceAlert >= 0 {
		m.cfg.MinBalanceAlert = newCfg.MinBalanceAlert
	}
	if newCfg.BalanceUSSD != "" {
		m.cfg.BalanceUSSD = newCfg.BalanceUSSD
	}
	if newCfg.OperatorPreset != "" {
		m.cfg.OperatorPreset = newCfg.OperatorPreset
	}
	slog.Info("tariff configuration updated",
		slog.String("modem", m.modemID),
		slog.Int("sms_limit", m.cfg.SMSLimit),
		slog.Float64("call_minutes_limit", m.cfg.CallMinutesLimit),
		slog.Int64("data_limit_mb", m.cfg.DataTrafficLimitMB),
		slog.Int("reset_day", m.cfg.ResetDayOfMonth),
	)
	m.persistLocked()
}

// UsageUpdate contains manual overrides for usage counters.
type UsageUpdate struct {
	SMSDayCount     *int
	SMSMonthCount   *int
	CallMinutesUsed *float64
	DataBytesUsed   *int64
}

// SetUsage manually overrides usage counters and persists the state.
func (m *Manager) SetUsage(update UsageUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if update.SMSDayCount != nil && *update.SMSDayCount >= 0 {
		m.smsDayCount = *update.SMSDayCount
	}
	if update.SMSMonthCount != nil && *update.SMSMonthCount >= 0 {
		m.smsMonthCount = *update.SMSMonthCount
	}
	if update.CallMinutesUsed != nil && *update.CallMinutesUsed >= 0 {
		m.callMinutesUsed = *update.CallMinutesUsed
	}
	if update.DataBytesUsed != nil && *update.DataBytesUsed >= 0 {
		m.dataBytesUsed = *update.DataBytesUsed
	}
	slog.Info("tariff usage counters manually updated",
		slog.String("modem", m.modemID),
		slog.Int("sms_month", m.smsMonthCount),
		slog.Float64("call_minutes", m.callMinutesUsed),
		slog.Int64("data_bytes", m.dataBytesUsed),
	)
	m.evaluateSMSLimits()
	m.evaluateCallLimits()
	m.evaluateDataLimits()
	m.persistLocked()
}

// GetConfig returns the current active tariff configuration.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// ResetQuotas zeroes monthly quota counters and resets warning flags.
func (m *Manager) ResetQuotas() {
	m.ResetMonthly()
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
	slog.Debug("tariff SMS usage recorded", slog.String("modem", m.modemID), slog.Int("count", count), slog.Int("day_total", m.smsDayCount), slog.Int("month_total", m.smsMonthCount))
	m.evaluateSMSLimits()
	m.persistLocked()
}

// RecordCallDuration increments voice call minutes from duration.
func (m *Manager) RecordCallDuration(d time.Duration) {
	if d <= 0 {
		return
	}
	m.RecordCallMinutes(d.Minutes())
}

// RecordCallMinutes increments voice call usage and checks quotas.
func (m *Manager) RecordCallMinutes(minutes float64) {
	if minutes <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callMinutesUsed += minutes
	slog.Debug("tariff call usage recorded", slog.String("modem", m.modemID), slog.Float64("minutes", minutes), slog.Float64("month_total", m.callMinutesUsed))
	m.evaluateCallLimits()
	m.persistLocked()
}

// RecordData records transmitted data bytes and checks quotas.
func (m *Manager) RecordData(bytes int64) {
	if bytes <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dataBytesUsed += bytes
	slog.Debug("tariff data usage recorded", slog.String("modem", m.modemID), slog.Int64("bytes", bytes), slog.Int64("month_total", m.dataBytesUsed))
	m.evaluateDataLimits()
	m.persistLocked()
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
	slog.Info("tariff balance updated", slog.String("modem", m.modemID), slog.Float64("balance", balance), slog.String("currency", m.currency))
	m.evaluateBalanceLimits(balance)
	m.persistLocked()
}

// ResetDaily zeroes the daily counter.
func (m *Manager) ResetDaily() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.smsDayCount = 0
	m.lastDailyResetDate = time.Now().UTC().Format("2006-01-02")
	m.persistLocked()
}

// ResetMonthly zeroes the monthly counter and resets warning flags.
func (m *Manager) ResetMonthly() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.smsMonthCount = 0
	m.warnedSMS90 = false
	m.warnedSMSExceeded = false
	m.callMinutesUsed = 0
	m.warnedCall90 = false
	m.warnedCallExceeded = false
	m.dataBytesUsed = 0
	m.warnedData90 = false
	m.warnedDataExceeded = false
	m.lastMonthlyResetMonth = time.Now().UTC().Format("2006-01")
	m.persistLocked()
}

// Status returns a copy of current usage statistics.
func (m *Manager) Status() UsageStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	remSMS := 0
	if m.cfg.SMSLimit > 0 {
		remSMS = m.cfg.SMSLimit - m.smsMonthCount
		if remSMS < 0 {
			remSMS = 0
		}
	}

	remMins := 0.0
	if m.cfg.CallMinutesLimit > 0 {
		remMins = m.cfg.CallMinutesLimit - m.callMinutesUsed
		if remMins < 0 {
			remMins = 0
		}
	}

	var limitBytes, remBytes int64
	if m.cfg.DataTrafficLimitMB > 0 {
		limitBytes = m.cfg.DataTrafficLimitMB * 1024 * 1024
		remBytes = limitBytes - m.dataBytesUsed
		if remBytes < 0 {
			remBytes = 0
		}
	}

	return UsageStatus{
		Balance:              m.balance,
		Currency:             m.currency,
		SMSDayCount:          m.smsDayCount,
		SMSMonthCount:        m.smsMonthCount,
		SMSLimit:             m.cfg.SMSLimit,
		SMSRemaining:         remSMS,
		CallMinutesLimit:     m.cfg.CallMinutesLimit,
		CallMinutesUsed:      m.callMinutesUsed,
		CallMinutesRemaining: remMins,
		DataTrafficLimitMB:   m.cfg.DataTrafficLimitMB,
		DataBytesLimit:       limitBytes,
		DataBytesUsed:        m.dataBytesUsed,
		DataBytesRemaining:   remBytes,
		LastBalanceCheck:     m.lastBalanceCheck,
		ResetDayOfMonth:      m.cfg.ResetDayOfMonth,
	}
}
