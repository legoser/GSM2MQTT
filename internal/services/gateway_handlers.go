package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
	}, func(a tariff.AlertEvent) {
		payload, _ := json.Marshal(a)
		_ = r.mqttClient.Publish(r.topics.AccountingAlert(), 1, false, payload)
		_ = r.mqttClient.Publish(r.topics.Alert(), 1, false, []byte(a.Message))
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
		_ = r.mqttClient.Publish(r.topics.USSDResponse(), 1, false, payload)
	})

	diagSvc := NewDiagnosticService(r.mCfg.ID, driver, func() (float64, string, error) {
		st := tariffMgr.Status()
		return st.Balance, st.Currency, nil
	}, func(alert string) {
		_ = r.mqttClient.Publish(r.topics.Alert(), 1, false, []byte(alert))
	})

	smsTracker := sms.NewTracker(r.cfg.SMS.DeliveryReport.Timeout, func(e sms.DeliveryEvent) {
		payload, _ := json.Marshal(e)
		_ = r.mqttClient.Publish(r.topics.SMSStatus(), 1, false, payload)
	})

	assembler := sms.NewAssembler(24 * time.Hour)
	sender := &atPDUSender{engine: engine}

	smsSvc := NewSMSService(SMSServiceConfig{
		ModemID:        r.mCfg.ID,
		Transliterate:  r.cfg.SMS.Encoding == "translit",
		DeliveryReport: r.cfg.SMS.DeliveryReport.Enabled,
	}, sender, filter, limiter, smsTracker, assembler, func(msg *sms.AssembledSMS) {
		slog.Info("incoming SMS received and processed",
			slog.String("modem", r.mCfg.ID),
			slog.String("from", msg.From),
			slog.String("text", msg.Text),
			slog.String("mqtt_topic", r.topics.SMSReceived()),
		)
		r.recordIncomingSMS(msg)
		metrics.DefaultRegistry.IncCounter("gsm2mqtt_sms_received_total", map[string]string{"modem": r.mCfg.ID})
		payload, _ := json.Marshal(msg)
		_ = r.mqttClient.Publish(r.topics.SMSReceived(), 1, true, payload)

		r.applyParsedBalance(msg.Text)
	})
	smsSvc.SetStorageManager(driver)

	callSvc := NewCallService(r.mCfg.ID, driver, func(e CallEvent) {
		metrics.DefaultRegistry.IncCounter("gsm2mqtt_calls_total", map[string]string{"modem": r.mCfg.ID, "type": e.Type})
		payload, _ := json.Marshal(e)
		if e.Type == "incoming" || e.Type == "ended" {
			_ = r.mqttClient.Publish(r.topics.CallIncoming(), 1, false, payload)
		} else if e.Type == "dtmf" {
			_ = r.mqttClient.Publish(r.topics.CallDTMF(), 1, false, payload)
		}
	})

	statusSvc := NewStatusService(StatusServiceConfig{
		ModemID:  r.mCfg.ID,
		Interval: r.cfg.Status.Interval,
	}, driver, func(rssi, dbm int) {
		metrics.DefaultRegistry.SetGauge("gsm2mqtt_signal_rssi", map[string]string{"modem": r.mCfg.ID}, float64(rssi))
		metrics.DefaultRegistry.SetGauge("gsm2mqtt_signal_dbm", map[string]string{"modem": r.mCfg.ID}, float64(dbm))
		payload := fmt.Sprintf(`{"rssi":%d,"dbm":%d}`, rssi, dbm)
		_ = r.mqttClient.Publish(r.topics.SignalStrength(), 1, false, []byte(payload))
	}, func(h ModemHealth) {
		r.updateHealth(h)
		stVal := 0.0
		if h.Status == "ready" {
			stVal = 1.0
		}
		metrics.DefaultRegistry.SetGauge("gsm2mqtt_modem_status", map[string]string{"modem": r.mCfg.ID}, stVal)
		payload, _ := json.Marshal(h)
		_ = r.mqttClient.Publish(r.topics.Health(), 1, false, payload)
	})

	return smsSvc, callSvc, ussdSvc, statusSvc, tariffMgr, diagSvc
}

func (r *ModemRunner) subscribeMQTT(
	
	smsSvc *SMSService,
	callSvc *CallService,
	ussdSvc *USSDService,
	tariffMgr *tariff.Manager,
	diagSvc *DiagnosticService,
	driver modem.Driver,
) {
	sanitizer := security.NewSanitizer(r.cfg.Security.AllowRawAT, r.cfg.Security.BlockedATCommands)

	_ = r.mqttClient.Subscribe(r.topics.SMSSend(), 1, func(_ string, payload []byte) {
		var req SendSMSRequest
		if err := json.Unmarshal(payload, &req); err == nil {
			refs, err := smsSvc.Send(context.Background(), req)
			if err != nil {
				metrics.DefaultRegistry.IncCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": r.mCfg.ID, "status": "failed"})
				slog.Error("sms send failure", slog.String("modem", r.mCfg.ID), slog.Any("error", err))
				if r.cfg.Tariff.AutoCheckOnError {
					rep, _ := diagSvc.RunDiagnostic(context.Background(), "sms_send_failure")
					if rep != nil {
						dPayload, _ := json.Marshal(rep)
						_ = r.mqttClient.Publish(r.topics.Diagnostic(), 1, false, dPayload)
					}
				}
			} else {
				metrics.DefaultRegistry.IncCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": r.mCfg.ID, "status": "sent"})
				tariffMgr.RecordSMS(len(refs))
				st := tariffMgr.Status()
				stPayload, _ := json.Marshal(st)
				_ = r.mqttClient.Publish(r.topics.AccountingStatus(), 1, false, stPayload)
			}
		}
	})

	_ = r.mqttClient.Subscribe(r.topics.CallDial(), 1, func(_ string, payload []byte) {
		var req struct {
			Number string `json:"number"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Number != "" {
			_ = callSvc.Dial(context.Background(), req.Number)
		}
	})

	_ = r.mqttClient.Subscribe(r.topics.CallHangup(), 1, func(_ string, _ []byte) {
		_ = callSvc.Hangup(context.Background())
	})

	_ = r.mqttClient.Subscribe(r.topics.USSDSend(), 1, func(_ string, payload []byte) {
		raw := strings.TrimSpace(string(payload))
		code := raw
		var req struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Code != "" {
			code = req.Code
		}
		code = strings.Trim(code, "\"")
		if code != "" {
			metrics.DefaultRegistry.IncCounter("gsm2mqtt_ussd_requests_total", map[string]string{"modem": r.mCfg.ID})
			resp, err := ussdSvc.Send(context.Background(), code)
			if err == nil && resp != nil {
				r.applyParsedBalance(resp.Message)
			}
		}
	})

	_ = r.mqttClient.Subscribe(r.topics.CommandRaw(), 1, func(_ string, payload []byte) {
		cmd := strings.TrimSpace(string(payload))
		if err := sanitizer.Validate(cmd); err != nil {
			_ = r.mqttClient.Publish(r.topics.CommandResponse(), 1, false, []byte(fmt.Sprintf("REJECTED: %v", err)))
			return
		}
		out, err := driver.SendRawAT(cmd)
		if err != nil {
			_ = r.mqttClient.Publish(r.topics.CommandResponse(), 1, false, []byte(fmt.Sprintf("ERROR: %v", err)))
		} else {
			_ = r.mqttClient.Publish(r.topics.CommandResponse(), 1, false, []byte(out))
		}
	})
}

func (r *ModemRunner) startBalanceLoop(ctx context.Context) {
	if !r.cfg.Tariff.Enabled {
		return
	}

	interval := r.cfg.Tariff.CheckInterval
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial balance check
	r.checkBalance(ctx)

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
	if !r.lastBalanceCheck.IsZero() && time.Since(r.lastBalanceCheck) < 1*time.Hour {
		r.mu.Unlock()
		slog.Debug("automatic balance check throttled (min 1 hour between checks)", slog.String("modem", r.mCfg.ID))
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

