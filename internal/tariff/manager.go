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

func (m *Manager) restoreFromStore() {
	if m.store == nil {
		return
	}
	state, err := m.store.Load(m.modemID)
	if err != nil || state == nil {
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	thisMonth := time.Now().UTC().Format("2006-01")

	m.balance = state.Balance
	if state.Currency != "" {
		m.currency = state.Currency
	}
	m.lastBalanceCheck = state.LastBalanceCheck
	m.dataBytesUsed = state.DataBytesUsed
	m.callMinutesUsed = state.CallMinutesUsed

	if state.LastDailyResetDate == today {
		m.smsDayCount = state.SMSDayCount
		m.lastDailyResetDate = state.LastDailyResetDate
	} else {
		m.smsDayCount = 0
		m.lastDailyResetDate = today
	}

	if state.LastMonthlyResetMonth == thisMonth {
		m.smsMonthCount = state.SMSMonthCount
		m.lastMonthlyResetMonth = state.LastMonthlyResetMonth
	} else {
		m.smsMonthCount = 0
		m.lastMonthlyResetMonth = thisMonth
	}
}

func (m *Manager) persistLocked() {
	if m.store == nil {
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	thisMonth := time.Now().UTC().Format("2006-01")
	if m.lastDailyResetDate == "" {
		m.lastDailyResetDate = today
	}
	if m.lastMonthlyResetMonth == "" {
		m.lastMonthlyResetMonth = thisMonth
	}

	_ = m.store.Save(m.modemID, State{
		Balance:               m.balance,
		Currency:              m.currency,
		SMSDayCount:           m.smsDayCount,
		SMSMonthCount:         m.smsMonthCount,
		CallMinutesUsed:       m.callMinutesUsed,
		DataBytesUsed:         m.dataBytesUsed,
		LastBalanceCheck:      m.lastBalanceCheck,
		LastDailyResetDate:    m.lastDailyResetDate,
		LastMonthlyResetMonth: m.lastMonthlyResetMonth,
	})
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
		DataBytesLimit:       limitBytes,
		DataBytesUsed:        m.dataBytesUsed,
		DataBytesRemaining:   remBytes,
		LastBalanceCheck:     m.lastBalanceCheck,
		ResetDayOfMonth:      m.cfg.ResetDayOfMonth,
	}
}
