package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/legoser/gsm2mqtt/internal/metrics"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

func (r *ModemRunner) subscribeMQTT(
	smsSvc *SMSService,
	callSvc *CallService,
	ussdSvc *USSDService,
	tariffMgr *tariff.Manager,
	diagSvc *DiagnosticService,
	driver modem.Driver,
) {
	r.subscribeSMS(smsSvc, callSvc, tariffMgr, diagSvc)
	r.subscribeCall(callSvc)
	r.subscribeUSSD(ussdSvc)
	r.subscribeRawAT(driver, r.sanitizer)
	r.subscribeTariff(tariffMgr)
}

func (r *ModemRunner) subscribeSMS(
	smsSvc *SMSService,
	callSvc *CallService,
	tariffMgr *tariff.Manager,
	diagSvc *DiagnosticService,
) {
	handler := func(_ string, payload []byte) {
		var req SendSMSRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			req = SendSMSRequest{Text: string(payload)}
		}
		text := req.GetText()
		if text == "" {
			return
		}

		targets := []string{req.To}
		if req.To == "" {
			if specificTo, remainingText, ok := extractLeadingRecipient(text); ok {
				targets = []string{specificTo}
				text = remainingText
			} else if r.recipientsMgr != nil {
				targets = r.recipientsMgr.Get()
			}
		}

		if len(targets) == 0 || (len(targets) == 1 && targets[0] == "") {
			slog.Warn("cannot send SMS: recipient is empty and no alert recipients configured", slog.String("modem", r.mCfg.ID))
			return
		}

		for _, target := range targets {
			if target != "" {
				r.sendAndReportSMS(smsSvc, callSvc, tariffMgr, diagSvc, req, target, text)
			}
		}
	}
	_ = r.mqttClient.Subscribe(r.topics.SMSSend(), 1, handler)
	if r.SlotIndex() == 1 {
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/sms/send", r.cfg.MQTT.TopicPrefix), 1, handler)
	}
}

func (r *ModemRunner) sendAndReportSMS(
	smsSvc *SMSService,
	callSvc *CallService,
	tariffMgr *tariff.Manager,
	diagSvc *DiagnosticService,
	req SendSMSRequest,
	target, text string,
) {
	sendReq := req
	sendReq.To = target
	sendReq.Text = text

	slog.Info("processing outgoing SMS request", slog.String("modem", r.mCfg.ID), slog.String("target", target))
	refs, err := smsSvc.Send(context.Background(), sendReq)
	if err != nil {
		metrics.DefaultRegistry.IncCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": r.mCfg.ID, "status": "failed"})
		slog.Error("sms send failure", slog.String("modem", r.mCfg.ID), slog.String("target", target), slog.Any("error", err))
		if r.cfg.Tariff.AutoCheckOnError {
			rep, _ := diagSvc.RunDiagnostic(context.Background(), "sms_send_failure")
			if rep != nil {
				dPayload, _ := json.Marshal(rep)
				_ = r.mqttClient.Publish(r.topics.Diagnostic(), 1, false, dPayload)
			}
		}
		if r.cfg.Security.FallbackCall && callSvc != nil {
			slog.Info("triggering fallback voice call after SMS failure", slog.String("modem", r.mCfg.ID), slog.String("target", target))
			go func(num string) {
				_ = callSvc.Dial(context.Background(), num)
			}(target)
		}
		return
	}

	metrics.DefaultRegistry.IncCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": r.mCfg.ID, "status": "sent"})
	tariffMgr.RecordSMS(len(refs))
	st := tariffMgr.Status()
	stPayload, _ := json.Marshal(st)
	_ = r.mqttClient.Publish(r.topics.AccountingStatus(), 1, false, stPayload)
}

func extractLeadingRecipient(text string) (string, string, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "8") && !strings.HasPrefix(trimmed, "7") {
		return "", text, false
	}
	idx := strings.IndexAny(trimmed, ": \t\n")
	if idx == -1 {
		return "", text, false
	}
	candidate := trimmed[:idx]
	norm, err := security.NormalizeNumber(candidate)
	if err != nil {
		return "", text, false
	}
	rest := strings.TrimSpace(trimmed[idx+1:])
	if rest == "" {
		return "", text, false
	}
	return norm, rest, true
}

func (r *ModemRunner) subscribeCall(callSvc *CallService) {
	dialHandler := func(_ string, payload []byte) {
		var req struct {
			Number string `json:"number"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Number != "" {
			slog.Info("call dial requested via MQTT", slog.String("modem", r.mCfg.ID), slog.String("number", req.Number))
			_ = callSvc.Dial(context.Background(), req.Number)
		}
	}
	hangupHandler := func(_ string, _ []byte) {
		slog.Info("call hangup requested via MQTT", slog.String("modem", r.mCfg.ID))
		_ = callSvc.Hangup(context.Background())
	}
	_ = r.mqttClient.Subscribe(r.topics.CallDial(), 1, dialHandler)
	_ = r.mqttClient.Subscribe(r.topics.CallHangup(), 1, hangupHandler)
	if r.SlotIndex() == 1 {
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/call/dial", r.cfg.MQTT.TopicPrefix), 1, dialHandler)
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/call/hangup", r.cfg.MQTT.TopicPrefix), 1, hangupHandler)
	}
}

func (r *ModemRunner) subscribeUSSD(ussdSvc *USSDService) {
	handler := func(_ string, payload []byte) {
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
			slog.Info("USSD query requested via MQTT", slog.String("modem", r.mCfg.ID), slog.String("code", code))
			metrics.DefaultRegistry.IncCounter("gsm2mqtt_ussd_requests_total", map[string]string{"modem": r.mCfg.ID})
			resp, err := ussdSvc.Send(context.Background(), code)
			if err == nil && resp != nil {
				r.applyParsedBalance(resp.Message)
			}
		}
	}
	_ = r.mqttClient.Subscribe(r.topics.USSDSend(), 1, handler)
	if r.SlotIndex() == 1 {
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/ussd/send", r.cfg.MQTT.TopicPrefix), 1, handler)
	}
}

func (r *ModemRunner) subscribeRawAT(driver modem.Driver, sanitizer *security.Sanitizer) {
	handler := func(_ string, payload []byte) {
		cmd := strings.TrimSpace(string(payload))
		slog.Info("raw AT command requested via MQTT", slog.String("modem", r.mCfg.ID), slog.String("cmd", cmd))
		if err := sanitizer.Validate(cmd); err != nil {
			slog.Warn("raw AT command rejected by sanitizer", slog.String("modem", r.mCfg.ID), slog.String("cmd", cmd), slog.Any("error", err))
			_ = r.mqttClient.Publish(r.topics.CommandResponse(), 1, false, []byte(fmt.Sprintf("REJECTED: %v", err)))
			return
		}
		out, err := driver.SendRawAT(cmd)
		if err != nil {
			slog.Error("raw AT command execution failed", slog.String("modem", r.mCfg.ID), slog.String("cmd", cmd), slog.Any("error", err))
			_ = r.mqttClient.Publish(r.topics.CommandResponse(), 1, false, []byte(fmt.Sprintf("ERROR: %v", err)))
		} else {
			_ = r.mqttClient.Publish(r.topics.CommandResponse(), 1, false, []byte(out))
		}
	}
	_ = r.mqttClient.Subscribe(r.topics.CommandRaw(), 1, handler)
	if r.SlotIndex() == 1 {
		_ = r.mqttClient.Subscribe(fmt.Sprintf("%s/modem/gsm_modem/command/raw", r.cfg.MQTT.TopicPrefix), 1, handler)
	}
}
