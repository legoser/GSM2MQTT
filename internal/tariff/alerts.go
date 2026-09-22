package tariff

import (
	"fmt"
)

func (m *Manager) evaluateSMSLimits() {
	if m.cfg.SMSLimit <= 0 || m.onAlert == nil {
		return
	}

	ratio := float64(m.smsMonthCount) / float64(m.cfg.SMSLimit)

	if ratio >= 0.9 && ratio < 1.0 && !m.warnedSMS90 {
		m.warnedSMS90 = true
		m.onAlert(AlertEvent{
			Type:    "sms_limit_warning",
			Message: fmt.Sprintf("90%% of monthly SMS quota reached (%d/%d)", m.smsMonthCount, m.cfg.SMSLimit),
			Value:   float64(m.smsMonthCount),
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedSMSExceeded {
		m.warnedSMSExceeded = true
		m.onAlert(AlertEvent{
			Type:    "sms_limit_exceeded",
			Message: fmt.Sprintf("Monthly SMS quota exceeded (%d/%d)", m.smsMonthCount, m.cfg.SMSLimit),
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
		m.onAlert(AlertEvent{
			Type:    "call_limit_warning",
			Message: fmt.Sprintf("90%% of call minutes quota reached (%.1f/%.1f)", m.callMinutesUsed, m.cfg.CallMinutesLimit),
			Value:   m.callMinutesUsed,
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedCallExceeded {
		m.warnedCallExceeded = true
		m.onAlert(AlertEvent{
			Type:    "call_limit_exceeded",
			Message: fmt.Sprintf("Call minutes quota exceeded (%.1f/%.1f)", m.callMinutesUsed, m.cfg.CallMinutesLimit),
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
		m.onAlert(AlertEvent{
			Type:    "data_limit_warning",
			Message: fmt.Sprintf("90%% of data traffic quota reached (%.1f MB/%d MB)", float64(m.dataBytesUsed)/(1024*1024), m.cfg.DataTrafficLimitMB),
			Value:   float64(m.dataBytesUsed),
			ModemID: m.modemID,
		})
	}

	if ratio >= 1.0 && !m.warnedDataExceeded {
		m.warnedDataExceeded = true
		m.onAlert(AlertEvent{
			Type:    "data_limit_exceeded",
			Message: fmt.Sprintf("Data traffic quota exceeded (%.1f MB/%d MB)", float64(m.dataBytesUsed)/(1024*1024), m.cfg.DataTrafficLimitMB),
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
