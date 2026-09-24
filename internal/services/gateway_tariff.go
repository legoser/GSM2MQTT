package services

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/legoser/gsm2mqtt/internal/metrics"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

func (r *ModemRunner) subscribeTariff(tariffMgr *tariff.Manager) {
	if tariffMgr == nil {
		return
	}

	setHandler := func(_ string, payload []byte) {
		var req struct {
			SMSLimit           *int     `json:"sms_limit"`
			CallMinutesLimit   *float64 `json:"call_minutes_limit"`
			DataTrafficLimitMB *int64   `json:"data_traffic_limit_mb"`
			ResetDayOfMonth    *int     `json:"reset_day_of_month"`
			MinBalanceAlert    *float64 `json:"min_balance_alert"`
			BalanceUSSD        *string  `json:"balance_ussd"`
			OperatorPreset     *string  `json:"operator_preset"`
			SMSMonthCount      *int     `json:"sms_month_count"`
			SMSDayCount        *int     `json:"sms_day_count"`
			CallMinutesUsed    *float64 `json:"call_minutes_used"`
			DataBytesUsed      *int64   `json:"data_bytes_used"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			slog.Warn("invalid tariff config json received via MQTT", slog.String("modem", r.mCfg.ID), slog.Any("error", err))
			return
		}
		current := tariffMgr.GetConfig()
		if req.SMSLimit != nil {
			current.SMSLimit = *req.SMSLimit
		}
		if req.CallMinutesLimit != nil {
			current.CallMinutesLimit = *req.CallMinutesLimit
		}
		if req.DataTrafficLimitMB != nil {
			current.DataTrafficLimitMB = *req.DataTrafficLimitMB
		}
		if req.ResetDayOfMonth != nil {
			current.ResetDayOfMonth = *req.ResetDayOfMonth
		}
		if req.MinBalanceAlert != nil {
			current.MinBalanceAlert = *req.MinBalanceAlert
		}
		if req.BalanceUSSD != nil {
			current.BalanceUSSD = *req.BalanceUSSD
		}
		if req.OperatorPreset != nil {
			current.OperatorPreset = *req.OperatorPreset
		}
		tariffMgr.UpdateConfig(current)

		if req.SMSMonthCount != nil || req.SMSDayCount != nil || req.CallMinutesUsed != nil || req.DataBytesUsed != nil {
			tariffMgr.SetUsage(tariff.UsageUpdate{
				SMSDayCount:     req.SMSDayCount,
				SMSMonthCount:   req.SMSMonthCount,
				CallMinutesUsed: req.CallMinutesUsed,
				DataBytesUsed:   req.DataBytesUsed,
			})
		}

		r.publishAccountingStatus(tariffMgr)
	}

	resetHandler := func(_ string, _ []byte) {
		slog.Info("tariff quota reset requested via MQTT", slog.String("modem", r.mCfg.ID))
		tariffMgr.ResetQuotas()
		r.publishAccountingStatus(tariffMgr)
	}

	_ = r.mqttClient.Subscribe(r.topics.TariffSet(), 1, setHandler)
	_ = r.mqttClient.Subscribe(r.topics.TariffReset(), 1, resetHandler)
	if r.SlotIndex() == 1 {
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/tariff/set", r.cfg.MQTT.TopicPrefix), 1, setHandler)
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/tariff/reset", r.cfg.MQTT.TopicPrefix), 1, resetHandler)
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/tariff/set", r.cfg.MQTT.TopicPrefix), 1, setHandler)
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/tariff/reset", r.cfg.MQTT.TopicPrefix), 1, resetHandler)
	}
}

func (r *ModemRunner) publishAccountingStatus(tariffMgr *tariff.Manager) {
	if tariffMgr == nil || r.mqttClient == nil || !r.mqttClient.IsConnected() {
		return
	}
	st := tariffMgr.Status()
	metrics.DefaultRegistry.SetGauge("gsm2mqtt_tariff_sms_used", map[string]string{"modem": r.mCfg.ID}, float64(st.SMSMonthCount))
	metrics.DefaultRegistry.SetGauge("gsm2mqtt_tariff_sms_limit", map[string]string{"modem": r.mCfg.ID}, float64(st.SMSLimit))
	stPayload, _ := json.Marshal(st)
	_ = r.mqttClient.Publish(r.topics.AccountingStatus(), 1, true, stPayload)
}

// UpdateTariffConfig updates the active tariff parameters and publishes new state.
func (r *ModemRunner) UpdateTariffConfig(cfg tariff.Config) error {
	r.mu.RLock()
	tm := r.tariffMgr
	r.mu.RUnlock()
	if tm == nil {
		return fmt.Errorf("tariff manager not initialized")
	}
	tm.UpdateConfig(cfg)
	r.publishAccountingStatus(tm)
	return nil
}

// SetTariffUsage manually updates tariff usage counters and publishes new state.
func (r *ModemRunner) SetTariffUsage(update tariff.UsageUpdate) error {
	r.mu.RLock()
	tm := r.tariffMgr
	r.mu.RUnlock()
	if tm == nil {
		return fmt.Errorf("tariff manager not initialized")
	}
	tm.SetUsage(update)
	r.publishAccountingStatus(tm)
	return nil
}

// ResetTariffQuotas resets monthly quota counters and publishes new state.
func (r *ModemRunner) ResetTariffQuotas() error {
	r.mu.RLock()
	tm := r.tariffMgr
	r.mu.RUnlock()
	if tm == nil {
		return fmt.Errorf("tariff manager not initialized")
	}
	tm.ResetQuotas()
	r.publishAccountingStatus(tm)
	return nil
}

// GetTariffStatus returns current tariff usage statistics.
func (r *ModemRunner) GetTariffStatus() (*tariff.UsageStatus, error) {
	r.mu.RLock()
	tm := r.tariffMgr
	r.mu.RUnlock()
	if tm == nil {
		return nil, fmt.Errorf("tariff manager not initialized")
	}
	st := tm.Status()
	return &st, nil
}
