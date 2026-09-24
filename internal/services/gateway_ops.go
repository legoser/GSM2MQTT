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
	"github.com/legoser/gsm2mqtt/internal/modem/drivers"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/operator"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

// ID returns the configured identifier of the modem.
func (r *ModemRunner) ID() string {
	return r.mCfg.ID
}

// Status returns the operational status of the modem.
func (r *ModemRunner) Status() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.lastHealth.Status == "" {
		return "ready"
	}
	return r.lastHealth.Status
}

// Signal returns the CSQ signal strength RSSI.
func (r *ModemRunner) Signal() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastHealth.Signal
}

// Operator returns the detected network operator name.
func (r *ModemRunner) Operator() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastHealth.Operator
}

// Dial initiates a voice call on this modem.
func (r *ModemRunner) Dial(ctx context.Context, number string) error {
	r.mu.RLock()
	svc := r.callSvc
	r.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("call service not initialized")
	}
	return svc.Dial(ctx, number)
}

// Hangup terminates any active voice call on this modem.
func (r *ModemRunner) Hangup(ctx context.Context) error {
	r.mu.RLock()
	svc := r.callSvc
	r.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("call service not initialized")
	}
	return svc.Hangup(ctx)
}

// SendRawAT sends an arbitrary AT command directly through the modem driver.
func (r *ModemRunner) SendRawAT(ctx context.Context, cmd string) (string, error) {
	if err := r.sanitizer.Validate(cmd); err != nil {
		return "", err
	}

	r.mu.RLock()
	drv := r.driver
	r.mu.RUnlock()
	if drv == nil {
		return "", fmt.Errorf("modem driver not initialized")
	}
	return drv.SendRawAT(cmd)
}

func (r *ModemRunner) applyParsedBalance(text string) {
	bal, err := operator.ParseBalance(text, r.cfg.Tariff.BalanceRegex)
	if err != nil {
		return
	}

	currency := r.lastCurrency
	if currency == "" {
		currency = tariff.DefaultCurrency
	}

	r.mu.Lock()
	r.lastBalance = bal
	r.lastCurrency = currency
	tm := r.tariffMgr
	r.mu.Unlock()

	if tm != nil {
		tm.UpdateBalance(bal, currency)
	}

	metrics.DefaultRegistry.SetGauge("gsm2mqtt_balance_rub", map[string]string{"modem": r.mCfg.ID}, bal)
	if r.mqttClient != nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(r.topics.Balance(), 1, false, []byte(fmt.Sprintf("%.2f", bal)))
		if tm != nil {
			st := tm.Status()
			metrics.DefaultRegistry.SetGauge("gsm2mqtt_tariff_sms_used", map[string]string{"modem": r.mCfg.ID}, float64(st.SMSMonthCount))
			metrics.DefaultRegistry.SetGauge("gsm2mqtt_tariff_sms_limit", map[string]string{"modem": r.mCfg.ID}, float64(st.SMSLimit))
			stPayload, _ := json.Marshal(st)
			_ = r.mqttClient.Publish(r.topics.AccountingStatus(), 1, false, stPayload)
		}
	}
	slog.Info("modem balance updated", slog.String("modem", r.mCfg.ID), slog.Float64("balance", bal), slog.String("currency", currency))
}

func (r *ModemRunner) createDriver(engine *at.Engine) modem.Driver {
	modemType := strings.ToLower(r.mCfg.Type)
	if modemType == "" || modemType == "auto" {
		detectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		detected, err := modem.Detect(detectCtx, engine)
		cancel()
		if err == nil && detected != "" && detected != modem.TypeGeneric {
			slog.Info("auto-detected modem type", slog.String("modem", r.mCfg.ID), slog.String("detected", string(detected)))
			modemType = string(detected)
		}
	}

	switch modemType {
	case "siemens", "tc35", "tc35i", "mc35", "mc35i", "mc55", "tc65", "cinterion":
		return drivers.NewSiemensDriver(engine)
	case "simcom", "sim800", "sim800l", "sim800c", "sim900", "sim7000", "sim7600":
		return drivers.NewSIMComDriver(engine)
	case "huawei":
		return drivers.NewHuaweiDriver(engine)
	case "neoway", "m590", "m590e", "m580":
		return drivers.NewNeowayDriver(engine)
	default:
		return drivers.NewGenericDriver(engine)
	}
}

// SetSlotIndex sets the 1-based modem slot index for Home Assistant auto-discovery.
func (r *ModemRunner) SetSlotIndex(index int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index <= 0 {
		index = 1
	}
	r.slotIndex = index
}

// SlotIndex returns the 1-based modem slot index.
func (r *ModemRunner) SlotIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.slotIndex
}

// SetRecipientsManager registers the dynamic alert recipients manager.
func (r *ModemRunner) SetRecipientsManager(mgr *security.RecipientsManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recipientsMgr = mgr
}

func (r *ModemRunner) publishDiscovery(driver modem.Driver) {
	if !r.cfg.MQTT.Discovery {
		return
	}
	info, _ := driver.Identify()
	mfg := ""
	model := ""
	sw := ""
	if info != nil {
		mfg = info.Manufacturer
		model = info.Model
		sw = info.Revision
	}
	if mfg == "" || strings.EqualFold(mfg, "undefined") {
		mfg = "Unknown"
	}
	if model == "" {
		model = r.mCfg.Type
	}

	if gwMsg, err := mqtt.BuildGatewayDiscovery(r.cfg.MQTT.DiscoveryPrefix, r.cfg.MQTT.TopicPrefix, "1.0.0"); err == nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(gwMsg.Topic, 1, true, gwMsg.Payload)
	}
	if mcMsg, err := mqtt.BuildGatewayModemCountDiscovery(r.cfg.MQTT.DiscoveryPrefix, r.cfg.MQTT.TopicPrefix, "1.0.0"); err == nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(mcMsg.Topic, 1, true, mcMsg.Payload)
	}
	if amMsg, err := mqtt.BuildGatewayActiveModemDiscovery(r.cfg.MQTT.DiscoveryPrefix, r.cfg.MQTT.TopicPrefix, "1.0.0"); err == nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(amMsg.Topic, 1, true, amMsg.Payload)
	}

	gwModemsPayload, _ := json.Marshal(map[string]any{
		"count":        1,
		"active_modem": model,
		"modems": []map[string]any{
			{
				"id":           r.mCfg.ID,
				"model":        model,
				"manufacturer": mfg,
				"port":         r.mCfg.Port,
				"status":       "ready",
			},
		},
	})
	_ = r.mqttClient.Publish(fmt.Sprintf("%s/gateway/modems", r.cfg.MQTT.TopicPrefix), 1, true, gwModemsPayload)

	if recMsg, err := mqtt.BuildRecipientsTextDiscovery(r.cfg.MQTT.DiscoveryPrefix, r.cfg.MQTT.TopicPrefix); err == nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(recMsg.Topic, 1, true, recMsg.Payload)
	}

	if r.recipientsMgr != nil && r.mqttClient.IsConnected() {
		recPayload, _ := json.Marshal(r.recipientsMgr.Get())
		_ = r.mqttClient.Publish(fmt.Sprintf("%s/config/recipients", r.cfg.MQTT.TopicPrefix), 1, true, recPayload)
	}

	messages, err := mqtt.BuildModemDiscoveries(mqtt.ModemDiscoveryParams{
		DiscoveryPrefix: r.cfg.MQTT.DiscoveryPrefix,
		TopicPrefix:     r.cfg.MQTT.TopicPrefix,
		ModemID:         r.mCfg.ID,
		Manufacturer:    mfg,
		Model:           model,
		SwVersion:       sw,
		Currency:        r.lastCurrency,
		SlotIndex:       r.SlotIndex(),
	})
	if err == nil && r.mqttClient.IsConnected() {
		for _, msg := range messages {
			_ = r.mqttClient.Publish(msg.Topic, 1, true, msg.Payload)
		}
	}

	r.mu.RLock()
	tm := r.tariffMgr
	r.mu.RUnlock()
	if tm != nil && r.mqttClient.IsConnected() {
		st := tm.Status()
		stPayload, _ := json.Marshal(st)
		_ = r.mqttClient.Publish(r.topics.AccountingStatus(), 1, true, stPayload)
	}
}

func (r *ModemRunner) startStorageCheckLoop(ctx context.Context, smsSvc *SMSService) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cap, err := smsSvc.CheckStorageCapacity()
			if err == nil && cap != nil && cap.Used > 0 {
				slog.Info("unprocessed SMS detected in storage, triggering sync & purge",
					slog.String("modem", r.mCfg.ID),
					slog.String("storage", cap.Name),
					slog.Int("used", cap.Used),
					slog.Int("total", cap.Total),
				)
				_, _ = smsSvc.SyncStoredMessages(ctx, "SM", "ME")
			}
		}
	}
}
