package tariff

import (
	"time"
)

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

	if state.LastDailyResetDate == "" || state.LastDailyResetDate == today {
		m.smsDayCount = state.SMSDayCount
		m.lastDailyResetDate = today
	} else {
		m.smsDayCount = 0
		m.lastDailyResetDate = today
	}

	if state.LastMonthlyResetMonth == "" || state.LastMonthlyResetMonth == thisMonth {
		m.smsMonthCount = state.SMSMonthCount
		m.lastMonthlyResetMonth = thisMonth
	} else {
		m.smsMonthCount = 0
		m.lastMonthlyResetMonth = thisMonth
	}

	if state.SMSLimit > 0 {
		m.cfg.SMSLimit = state.SMSLimit
	}
	if state.CallMinutesLimit > 0 {
		m.cfg.CallMinutesLimit = state.CallMinutesLimit
	}
	if state.DataTrafficLimitMB > 0 {
		m.cfg.DataTrafficLimitMB = state.DataTrafficLimitMB
	}
	if state.ResetDayOfMonth > 0 && state.ResetDayOfMonth <= 31 {
		m.cfg.ResetDayOfMonth = state.ResetDayOfMonth
	}
	if state.MinBalanceAlert > 0 {
		m.cfg.MinBalanceAlert = state.MinBalanceAlert
	}
	if state.BalanceUSSD != "" {
		m.cfg.BalanceUSSD = state.BalanceUSSD
	}
	if state.OperatorPreset != "" {
		m.cfg.OperatorPreset = state.OperatorPreset
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
		SMSLimit:              m.cfg.SMSLimit,
		CallMinutesLimit:      m.cfg.CallMinutesLimit,
		DataTrafficLimitMB:    m.cfg.DataTrafficLimitMB,
		ResetDayOfMonth:       m.cfg.ResetDayOfMonth,
		MinBalanceAlert:       m.cfg.MinBalanceAlert,
		BalanceUSSD:           m.cfg.BalanceUSSD,
		OperatorPreset:        m.cfg.OperatorPreset,
	})
}
