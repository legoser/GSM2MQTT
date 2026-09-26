package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/legoser/gsm2mqtt/internal/metrics"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/operator"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/sms"
	"github.com/legoser/gsm2mqtt/internal/tariff"
	"github.com/legoser/gsm2mqtt/internal/ussd"
)

func (r *ModemRunner) wireServices(
	engine *at.Engine,
	driver modem.Driver,

) (*SMSService, *CallService, *USSDService, *StatusService, *tariff.Manager, *DiagnosticService) {
	filter := security.NewFilter(r.cfg.Security.IncomingFilter, r.cfg.Security.Whitelist, r.cfg.Security.Blacklist)
	limiter := security.NewRateLimiterWithConfig(security.RateLimiterConfig{
		Enabled:             r.cfg.Security.RateLimit.Enabled,
		MaxPerMinute:        r.cfg.Security.RateLimit.MaxSMSPerMinute,
		MaxPerHour:          r.cfg.Security.RateLimit.MaxSMSPerHour,
		MaxPerDay:           r.cfg.Security.RateLimit.MaxSMSPerDay,
		MaxPerNumberPerHour: r.cfg.Security.RateLimit.MaxSMSPerNumberPerHour,
		Cooldown:            time.Duration(r.cfg.Security.RateLimit.CooldownMinutes) * time.Minute,
	})

	tariffMgr := tariff.NewManager(r.mCfg.ID, tariff.Config{
		Enabled:            r.cfg.Tariff.Enabled,
		OperatorPreset:     r.cfg.Tariff.OperatorPreset,
		BalanceUSSD:        r.cfg.Tariff.BalanceUSSD,
		BalanceRegex:       r.cfg.Tariff.BalanceRegex,
		AutoCheckOnError:   r.cfg.Tariff.AutoCheckOnError,
		CheckInterval:      r.cfg.Tariff.CheckInterval,
		MinBalanceAlert:    r.cfg.Tariff.MinBalanceAlert,
		SMSLimit:           r.cfg.Tariff.SMSLimit,
		CallMinutesLimit:   r.cfg.Tariff.CallMinutesLimit,
		DataTrafficLimitMB: r.cfg.Tariff.DataTrafficLimitMB,
		ResetDayOfMonth:    r.cfg.Tariff.ResetDayOfMonth,
		StorageDir:         r.cfg.Tariff.StorageDir,
		Location:           r.location(),
	}, func(a tariff.AlertEvent) {
		payload, _ := json.Marshal(a)
		_ = r.mqttClient.Publish(r.topics.AccountingAlert(), r.qos(), false, payload)
		_ = r.mqttClient.Publish(r.topics.Alert(), r.qos(), false, []byte(a.Message))
		r.publishEvent(NewEvent(
			r.mCfg.ID,
			a.Type,
			EventCategoryTariff,
			EventLevelWarning,
			a.Message,
			r.location(),
			map[string]any{
				"value":     a.Value,
				"threshold": r.cfg.Tariff.MinBalanceAlert,
			},
		))
	})

	r.mu.Lock()
	if r.lastBalance != 0 {
		tariffMgr.UpdateBalance(r.lastBalance, r.lastCurrency)
	} else {
		st := tariffMgr.Status()
		if st.Balance != 0 {
			r.lastBalance = st.Balance
			r.lastCurrency = st.Currency
		}
	}
	r.mu.Unlock()

	ussdSvc := NewUSSDService(r.mCfg.ID, driver, func(resp *ussd.Response) {
		payload, _ := json.Marshal(resp)
		_ = r.mqttClient.Publish(r.topics.USSDResponse(), r.qos(), false, payload)
	})

	diagSvc := NewDiagnosticService(r.mCfg.ID, driver, func() (float64, string, error) {
		st := tariffMgr.Status()
		return st.Balance, st.Currency, nil
	}, func(alert string) {
		_ = r.mqttClient.Publish(r.topics.Alert(), r.qos(), false, []byte(alert))
	})

	smsTracker := sms.NewTracker(r.cfg.SMS.DeliveryReport.Timeout, func(e sms.DeliveryEvent) {
		payload, _ := json.Marshal(e)
		_ = r.mqttClient.Publish(r.topics.SMSStatus(), r.qos(), false, payload)
	})

	asmTimeout := r.cfg.SMS.AssemblyTimeout
	if asmTimeout <= 0 {
		asmTimeout = 30 * time.Second
	}
	assembler := sms.NewAssembler(24*time.Hour, asmTimeout)
	sender := &atPDUSender{engine: engine}

	smsSvc := NewSMSService(SMSServiceConfig{
		ModemID:        r.mCfg.ID,
		Transliterate:  r.cfg.SMS.Encoding == "translit",
		DeliveryReport: r.cfg.SMS.DeliveryReport.Enabled,
	}, sender, filter, limiter, smsTracker, assembler, func(msg *sms.AssembledSMS) {
		slog.Info("incoming SMS received and processed",
			slog.String("modem", r.mCfg.ID),
			slog.String("from", "[REDACTED]"),
			slog.String("text", "[REDACTED]"),
			slog.String("mqtt_topic", r.topics.SMSReceived()),
		)
		r.recordIncomingSMS(msg)
		metrics.DefaultRegistry.IncCounter("gsm2mqtt_sms_received_total", map[string]string{"modem": r.mCfg.ID})
		payload, _ := json.Marshal(msg)
		_ = r.mqttClient.Publish(r.topics.SMSReceived(), r.qos(), false, payload)
		_ = r.mqttClient.Publish(r.topics.SMSLast(), r.qos(), true, payload)

		r.applyParsedBalance(msg.Text)
	})
	smsSvc.SetStorageManager(driver)

	callSvc := NewCallService(r.mCfg.ID, driver, func(e CallEvent) {
		metrics.DefaultRegistry.IncCounter("gsm2mqtt_calls_total", map[string]string{"modem": r.mCfg.ID, "type": e.Type})

		if e.Type == "ended" && e.Duration > 3*time.Second {
			minutes := math.Ceil(e.Duration.Seconds() / 60.0)
			tariffMgr.RecordCallMinutes(minutes)
			r.publishAccountingStatus(tariffMgr)
		}

		payload, _ := json.Marshal(e)
		switch e.Type {
		case "incoming", "ended":
			_ = r.mqttClient.Publish(r.topics.CallIncoming(), r.qos(), false, payload)
		case "dtmf":
			_ = r.mqttClient.Publish(r.topics.CallDTMF(), r.qos(), false, payload)
		}
	})

	statusSvc := NewStatusService(StatusServiceConfig{
		ModemID:  r.mCfg.ID,
		Interval: r.cfg.Status.Interval,
	}, driver, func(rssi, dbm int) {
		metrics.DefaultRegistry.SetGauge("gsm2mqtt_signal_rssi", map[string]string{"modem": r.mCfg.ID}, float64(rssi))
		metrics.DefaultRegistry.SetGauge("gsm2mqtt_signal_dbm", map[string]string{"modem": r.mCfg.ID}, float64(dbm))
		payload := fmt.Sprintf(`{"rssi":%d,"dbm":%d}`, rssi, dbm)
		_ = r.mqttClient.Publish(r.topics.SignalStrength(), r.qos(), false, []byte(payload))
	}, func(h ModemHealth) {
		h.ModemID = r.mCfg.ID
		r.mu.RLock()
		prevStatus := r.lastHealth.Status
		r.mu.RUnlock()

		r.updateHealth(h)
		stVal := 0.0
		if h.Status == "ready" {
			stVal = 1.0
		}
		metrics.DefaultRegistry.SetGauge("gsm2mqtt_modem_status", map[string]string{"modem": r.mCfg.ID}, stVal)
		payload, _ := json.Marshal(h)
		_ = r.mqttClient.Publish(r.topics.Health(), r.qos(), true, payload)

		if prevStatus != "" && prevStatus != h.Status {
			switch h.Status {
			case "ready":
				r.publishEvent(NewEvent(r.mCfg.ID, "modem_ready", EventCategoryHardware, EventLevelInfo, fmt.Sprintf("Modem %s is ready and operational", r.mCfg.ID), r.location(), nil))
			case "degraded":
				r.publishEvent(NewEvent(r.mCfg.ID, "modem_degraded", EventCategoryHardware, EventLevelWarning, fmt.Sprintf("Modem %s operational status degraded", r.mCfg.ID), r.location(), nil))
			case "not_ready", "error":
				r.publishEvent(NewEvent(r.mCfg.ID, "modem_error", EventCategoryHardware, EventLevelError, fmt.Sprintf("Modem %s error status: %s", r.mCfg.ID, h.Status), r.location(), nil))
			}
		}
	})

	return smsSvc, callSvc, ussdSvc, statusSvc, tariffMgr, diagSvc
}

func (r *ModemRunner) startBalanceLoop(ctx context.Context) {
	if !r.cfg.Tariff.Enabled || r.cfg.Tariff.CheckInterval <= 0 {
		slog.Info("periodic balance checking is disabled", slog.String("modem", r.mCfg.ID))
		return
	}

	interval := r.cfg.Tariff.CheckInterval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial balance check only if balance was never queried
	r.mu.RLock()
	hasBalance := r.lastBalance != 0
	r.mu.RUnlock()
	if !hasBalance {
		r.checkBalance(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkBalance(ctx)
		}
	}
}

func (r *ModemRunner) checkBalance(ctx context.Context) {
	r.mu.Lock()
	minInterval := 1 * time.Hour
	if r.cfg.Tariff.CheckInterval > minInterval {
		minInterval = r.cfg.Tariff.CheckInterval
	}
	if !r.lastBalanceCheck.IsZero() && time.Since(r.lastBalanceCheck) < minInterval {
		r.mu.Unlock()
		slog.Debug("automatic balance check throttled", slog.String("modem", r.mCfg.ID))
		return
	}
	r.lastBalanceCheck = time.Now()
	r.mu.Unlock()

	ussdCode := r.cfg.Tariff.BalanceUSSD
	if ussdCode == "" {
		preset, err := operator.GetPreset(r.cfg.Tariff.OperatorPreset)
		if err == nil {
			ussdCode = preset.USSDCode
		} else {
			ussdCode = "*100#"
		}
	}

	resp, err := r.ussdSvc.Send(ctx, ussdCode)
	if err != nil || resp == nil {
		return
	}

	r.applyParsedBalance(resp.Message)
}
