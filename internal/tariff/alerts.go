package tariff

import (
	"fmt"
	"log/slog"
)

func (m *Manager) evaluateSMSLimits() {
	if m.cfg.SMSLimit <= 0 || m.onAlert == nil {
		return
	}

	ratio := float64(m.smsMonthCount) / float64(m.cfg.SMSLimit)

	if ratio >= 0.9 && ratio < 1.0 && !m.warnedSMS90 {
		m.warnedSMS90 = true
		msg := fmt.Sprintf("90%% of monthly SMS quota reached (%d/%d)", m.smsMonthCount, m.cfg.SMSLimit)
		slog.Warn("tariff quota warning: 90% reached", slog.String("modem", m.modemID), slog.String("quota", "sms"), slog.Int("used", m.smsMonthCount), slog.Int("limit", m.cfg.SMSLimit))
		m.onAlert(AlertEvent{
			Type:    "sms_limit_warning",
			Message: msg,
			Value:   float64(m.smsMonthCount),
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedSMSExceeded {
		m.warnedSMSExceeded = true
		msg := fmt.Sprintf("Monthly SMS quota exceeded (%d/%d)", m.smsMonthCount, m.cfg.SMSLimit)
		slog.Warn("tariff quota exceeded", slog.String("modem", m.modemID), slog.String("quota", "sms"), slog.Int("used", m.smsMonthCount), slog.Int("limit", m.cfg.SMSLimit))
		m.onAlert(AlertEvent{
			Type:    "sms_limit_exceeded",
			Message: msg,
			Value:   float64(m.smsMonthCount),
			ModemID: m.modemID,
		})
	}
}

func (m *Manager) evaluateCallLimits() {
	if m.cfg.CallMinutesLimit <= 0 || m.onAlert == nil {
		return
	}

	ratio := m.callMinutesUsed / m.cfg.CallMinutesLimit

	if ratio >= 0.9 && ratio < 1.0 && !m.warnedCall90 {
		m.warnedCall90 = true
		msg := fmt.Sprintf("90%% of call minutes quota reached (%.1f/%.1f)", m.callMinutesUsed, m.cfg.CallMinutesLimit)
		slog.Warn("tariff quota warning: 90% reached", slog.String("modem", m.modemID), slog.String("quota", "call_minutes"), slog.Float64("used", m.callMinutesUsed), slog.Float64("limit", m.cfg.CallMinutesLimit))
		m.onAlert(AlertEvent{
			Type:    "call_limit_warning",
			Message: msg,
			Value:   m.callMinutesUsed,
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedCallExceeded {
		m.warnedCallExceeded = true
		msg := fmt.Sprintf("Call minutes quota exceeded (%.1f/%.1f)", m.callMinutesUsed, m.cfg.CallMinutesLimit)
		slog.Warn("tariff quota exceeded", slog.String("modem", m.modemID), slog.String("quota", "call_minutes"), slog.Float64("used", m.callMinutesUsed), slog.Float64("limit", m.cfg.CallMinutesLimit))
		m.onAlert(AlertEvent{
			Type:    "call_limit_exceeded",
			Message: msg,
			Value:   m.callMinutesUsed,
			ModemID: m.modemID,
		})
	}
}

func (m *Manager) evaluateDataLimits() {
	if m.cfg.DataTrafficLimitMB <= 0 || m.onAlert == nil {
		return
	}

	limitBytes := m.cfg.DataTrafficLimitMB * 1024 * 1024
	ratio := float64(m.dataBytesUsed) / float64(limitBytes)

	if ratio >= 0.9 && ratio < 1.0 && !m.warnedData90 {
		m.warnedData90 = true
		msg := fmt.Sprintf("90%% of data traffic quota reached (%.1f MB/%d MB)", float64(m.dataBytesUsed)/(1024*1024), m.cfg.DataTrafficLimitMB)
		slog.Warn("tariff quota warning: 90% reached", slog.String("modem", m.modemID), slog.String("quota", "data"), slog.Float64("used_mb", float64(m.dataBytesUsed)/(1024*1024)), slog.Int64("limit_mb", m.cfg.DataTrafficLimitMB))
		m.onAlert(AlertEvent{
			Type:    "data_limit_warning",
			Message: msg,
			Value:   float64(m.dataBytesUsed),
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedDataExceeded {
		m.warnedDataExceeded = true
		msg := fmt.Sprintf("Data traffic quota exceeded (%.1f MB/%d MB)", float64(m.dataBytesUsed)/(1024*1024), m.cfg.DataTrafficLimitMB)
		slog.Warn("tariff quota exceeded", slog.String("modem", m.modemID), slog.String("quota", "data"), slog.Float64("used_mb", float64(m.dataBytesUsed)/(1024*1024)), slog.Int64("limit_mb", m.cfg.DataTrafficLimitMB))
		m.onAlert(AlertEvent{
			Type:    "data_limit_exceeded",
			Message: msg,
			Value:   float64(m.dataBytesUsed),
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
			msg := fmt.Sprintf("Balance is low: %.2f %s (threshold: %.2f)", balance, m.currency, m.cfg.MinBalanceAlert)
			slog.Warn("tariff low balance warning", slog.String("modem", m.modemID), slog.Float64("balance", balance), slog.Float64("threshold", m.cfg.MinBalanceAlert), slog.String("currency", m.currency))
			m.onAlert(AlertEvent{
				Type:    "low_balance",
				Message: msg,
				Value:   balance,
				ModemID: m.modemID,
			})
		}
	} else {
		m.warnedLowBalance = false
	}
}
