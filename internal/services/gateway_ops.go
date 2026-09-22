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
	switch strings.ToLower(r.mCfg.Type) {
	case "siemens":
		return drivers.NewSiemensDriver(engine)
	case "simcom":
		return drivers.NewSIMComDriver(engine)
	case "huawei":
		return drivers.NewHuaweiDriver(engine)
	default:
		return drivers.NewGenericDriver(engine)
	}
}

func (r *ModemRunner) publishDiscovery(driver modem.Driver) {
	if !r.cfg.MQTT.Discovery {
		return
	}
	info, _ := driver.Identify()
	mfg := "Generic"
	model := "Modem"
	if info != nil {
		if info.Manufacturer != "" {
			mfg = info.Manufacturer
		}
		if info.Model != "" {
			model = info.Model
		}
	}
	disc, err := mqtt.BuildSignalDiscovery(r.cfg.MQTT.DiscoveryPrefix, r.cfg.MQTT.TopicPrefix, r.mCfg.ID, mfg, model)
	if err == nil && r.mqttClient.IsConnected() {
		_ = r.mqttClient.Publish(disc.Topic, 1, true, disc.Payload)
	}
}

func (r *ModemRunner) startStorageCheckLoop(ctx context.Context, smsSvc *SMSService) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cap, err := smsSvc.CheckStorageCapacity()
			if err == nil && cap != nil && cap.Total > 0 {
				ratio := float64(cap.Used) / float64(cap.Total)
				if ratio >= 0.8 {
					slog.Warn("SMS storage capacity high, triggering sync & purge",
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
}
