package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/tariff"
	"github.com/legoser/gsm2mqtt/internal/transport"
)

// ModemSummary represents the consolidated live state of a managed modem.
type ModemSummary struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Health   ModemHealth `json:"health"`
	Balance  float64     `json:"balance"`
	Currency string      `json:"currency"`
}

// ModemRunner manages the complete lifecycle, AT engine, and MQTT bridging for a single modem.
type ModemRunner struct {
	mCfg             config.ModemConfig
	cfg              *config.Config
	connector        *modem.Connector
	mqttClient       mqtt.MQTTClient
	topics           *mqtt.Topics
	mu               sync.RWMutex
	lastHealth       ModemHealth
	lastBalance      float64
	lastCurrency     string
	lastBalanceCheck time.Time
	driver           modem.Driver
	smsSvc           *SMSService
	ussdSvc          *USSDService
	callSvc          *CallService
	tariffMgr        *tariff.Manager
	receivedSMS      []ReceivedSMS
	slotIndex        int
	recipientsMgr    *security.RecipientsManager
	sanitizer        *security.Sanitizer
}

// DefaultCurrency defines standard currency when none is specified.
const DefaultCurrency = tariff.DefaultCurrency

// NewModemRunner instantiates an operational runner for a single modem configuration.
func NewModemRunner(
	mCfg config.ModemConfig,
	cfg *config.Config,
	connector *modem.Connector,
	mqttClient mqtt.MQTTClient,
) *ModemRunner {
	r := &ModemRunner{
		mCfg:         mCfg,
		cfg:          cfg,
		connector:    connector,
		mqttClient:   mqttClient,
		topics:       mqtt.NewTopics(cfg.MQTT.TopicPrefix, mCfg.ID),
		lastCurrency: DefaultCurrency,
		slotIndex:    1,
		sanitizer:    security.NewSanitizer(cfg.Security.AllowRawAT, cfg.Security.AllowedATCommands),
	}
	r.loadInbox()
	return r
}

// Run manages serial connection, automatically reconnects on error, and runs until context cancellation.
// Reconnects use exponential backoff with jitter (3s → 60s max) so a
// missing/unplugged modem does not hammer the port and the log.
func (r *ModemRunner) Run(ctx context.Context) error {
	const (
		minBackoff = 3 * time.Second
		maxBackoff = 60 * time.Second
	)
	backoff := minBackoff
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		started := time.Now()
		err := r.runOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A long-lived session means the failure is fresh: retry fast.
		// Only consecutive quick failures back off.
		if time.Since(started) > 5*time.Minute {
			backoff = minBackoff
		}
		r.updateHealth(ModemHealth{
			Status: "error",
			SIM:    "DISCONNECTED",
		})
		slog.Warn("modem port disconnected or unavailable, retrying...",
			slog.String("modem", r.mCfg.ID),
			slog.String("port", r.mCfg.Port),
			slog.Any("error", err),
			slog.Duration("retry_in", backoff),
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		// Exponential backoff with jitter: double up to max, ±20%.
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		backoff = backoff - time.Duration(rand.Int63n(int64(backoff)/5))
		if backoff < minBackoff {
			backoff = minBackoff
		}
	}
}

func (r *ModemRunner) runOnce(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	engine, closer, err := r.connector.Open(transport.PortConfig{
		Device:      r.mCfg.Port,
		BaudRate:    r.mCfg.BaudRate,
		DataBits:    r.mCfg.DataBits,
		StopBits:    r.mCfg.StopBits,
		Parity:      r.mCfg.Parity,
		FlowControl: r.mCfg.FlowControl,
	})
	if err != nil {
		return fmt.Errorf("failed to open modem port %s: %w", r.mCfg.Port, err)
	}
	defer closer()

	slog.Info("modem port opened successfully", slog.String("modem", r.mCfg.ID), slog.String("port", r.mCfg.Port))
	engineErrCh := make(chan error, 1)
	go func() {
		engineErrCh <- engine.Start(childCtx)
	}()

	driver := r.createDriver(engine)
	if err := driver.Init(childCtx); err != nil {
		slog.Warn("driver init completed with warning", slog.String("modem", r.mCfg.ID), slog.Any("error", err))
	} else {
		slog.Info("modem driver initialized", slog.String("modem", r.mCfg.ID), slog.String("type", r.mCfg.Type))
	}

	smsSvc, callSvc, ussdSvc, statusSvc, tariffMgr, diagSvc := r.wireServices(engine, driver)
	statusSvc.SetOnDisconnect(cancel)

	r.mu.Lock()
	r.driver = driver
	r.smsSvc = smsSvc
	r.ussdSvc = ussdSvc
	r.callSvc = callSvc
	r.tariffMgr = tariffMgr
	r.mu.Unlock()

	r.publishDiscovery(driver)

	go r.urcLoop(childCtx, engine, smsSvc, callSvc, ussdSvc)
	r.subscribeMQTT(childCtx, smsSvc, callSvc, ussdSvc, tariffMgr, diagSvc, driver)
	go statusSvc.Start(childCtx)
	go r.startBalanceLoop(childCtx)

	r.mu.RLock()
	if len(r.receivedSMS) > 0 && r.mqttClient != nil && r.mqttClient.IsConnected() {
		last := r.receivedSMS[len(r.receivedSMS)-1]
		lastPayload, _ := json.Marshal(map[string]any{
			"from":      last.Sender,
			"text":      last.Text,
			"timestamp": last.Timestamp,
		})
		_ = r.mqttClient.Publish(r.topics.SMSReceived(), 1, true, lastPayload)
	}
	r.mu.RUnlock()

	go func() {
		synced, err := smsSvc.SyncStoredMessages(childCtx, "SM", "ME")
		if err != nil {
			slog.Warn("stored SMS sync encountered error", slog.String("modem", r.mCfg.ID), slog.Any("error", err))
		} else if synced > 0 {
			slog.Info("stored offline SMS messages synced and purged", slog.String("modem", r.mCfg.ID), slog.Int("count", synced))
		}
	}()
	go r.startStorageCheckLoop(childCtx, smsSvc)

	slog.Info("modem runner is operational and listening", slog.String("modem", r.mCfg.ID), slog.String("port", r.mCfg.Port))

	select {
	case <-childCtx.Done():
		return fmt.Errorf("modem runner context cancelled")
	case <-ctx.Done():
		return ctx.Err()
	case err := <-engineErrCh:
		if err == nil {
			return fmt.Errorf("modem serial communication closed")
		}
		return fmt.Errorf("modem serial communication lost: %w", err)
	}
}

// Summary returns current live status snapshot for API inspection.
func (r *ModemRunner) Summary() ModemSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	balance := r.lastBalance
	currency := r.lastCurrency
	if currency == "" {
		currency = DefaultCurrency
	}
	if r.tariffMgr != nil {
		st := r.tariffMgr.Status()
		if st.Balance != 0 || r.lastBalance == 0 {
			balance = st.Balance
			currency = st.Currency
		}
	}

	return ModemSummary{
		ID:       r.mCfg.ID,
		Type:     r.mCfg.Type,
		Health:   r.lastHealth,
		Balance:  balance,
		Currency: currency,
	}
}

// SendSMS sends an SMS via the runner's SMS service.
func (r *ModemRunner) SendSMS(ctx context.Context, to, text string) ([]byte, error) {
	r.mu.RLock()
	svc := r.smsSvc
	tm := r.tariffMgr
	r.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("SMS service not initialized")
	}
	refs, err := svc.Send(ctx, SendSMSRequest{To: to, Text: text})
	if err == nil && tm != nil && len(refs) > 0 {
		tm.RecordSMS(len(refs))
		r.publishAccountingStatus(tm)
	}
	return refs, err
}

// SendUSSD executes a USSD query via the runner's USSD service.
func (r *ModemRunner) SendUSSD(ctx context.Context, code string) (string, error) {
	r.mu.RLock()
	svc := r.ussdSvc
	r.mu.RUnlock()
	if svc == nil {
		return "", fmt.Errorf("USSD service not initialized")
	}
	resp, err := svc.Send(ctx, code)
	if err != nil {
		return "", err
	}
	r.applyParsedBalance(resp.Message)
	return resp.Message, nil
}

// GetCallStatus returns the current voice call status for this modem runner.
func (r *ModemRunner) GetCallStatus() CallStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.callSvc != nil {
		return r.callSvc.Status()
	}
	return CallStatus{State: CallStateIdle, Message: "Idle"}
}

func (r *ModemRunner) updateHealth(h ModemHealth) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h.Operator == "" && r.lastHealth.Operator != "" {
		h.Operator = r.lastHealth.Operator
	}
	r.lastHealth = h
}

func (r *ModemRunner) urcLoop(
	ctx context.Context,
	engine *at.Engine,
	smsSvc *SMSService,
	callSvc *CallService,
	ussdSvc *USSDService,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-engine.URC():
			if !ok {
				return
			}
			smsSvc.HandleURC(line)
			callSvc.HandleURC(line)
			ussdSvc.HandleURC(line)
		}
	}
}
