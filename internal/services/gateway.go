package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/modem/drivers"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
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
	mCfg        config.ModemConfig
	cfg         *config.Config
	opener      transport.Opener
	mqttClient  mqtt.MQTTClient
	mu          sync.RWMutex
	lastHealth  ModemHealth
	smsSvc      *SMSService
	ussdSvc     *USSDService
	callSvc     *CallService
	tariffMgr   *tariff.Manager
	receivedSMS []ReceivedSMS
}

// NewModemRunner constructs a new ModemRunner.
func NewModemRunner(
	mCfg config.ModemConfig,
	cfg *config.Config,
	opener transport.Opener,
	mqttClient mqtt.MQTTClient,
) *ModemRunner {
	return &ModemRunner{
		mCfg:       mCfg,
		cfg:        cfg,
		opener:     opener,
		mqttClient: mqttClient,
	}
}

// Run manages serial connection, automatically reconnects on error, and runs until context cancellation.
func (r *ModemRunner) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := r.runOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.updateHealth(ModemHealth{
			Status: "error",
			SIM:    "DISCONNECTED",
		})
		slog.Warn("modem port disconnected or unavailable, retrying in 3s...",
			slog.String("modem", r.mCfg.ID),
			slog.String("port", r.mCfg.Port),
			slog.Any("error", err),
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func (r *ModemRunner) openPort() (transport.Port, error) {
	return r.opener.Open(transport.PortConfig{
		Device:      r.mCfg.Port,
		BaudRate:    r.mCfg.BaudRate,
		DataBits:    r.mCfg.DataBits,
		StopBits:    r.mCfg.StopBits,
		Parity:      r.mCfg.Parity,
		FlowControl: r.mCfg.FlowControl,
	})
}

func (r *ModemRunner) runOnce(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	port, err := r.openPort()
	if err != nil {
		return fmt.Errorf("failed to open modem port %s: %w", r.mCfg.Port, err)
	}
	defer port.Close()

	engine := at.NewEngine(port)
	engineErrCh := make(chan error, 1)
	go func() {
		engineErrCh <- engine.Start(childCtx)
	}()

	driver := r.createDriver(engine)
	if err := driver.Init(childCtx); err != nil {
		slog.Warn("driver init completed with warning", slog.String("modem", r.mCfg.ID), slog.Any("error", err))
	}

	topics := mqtt.NewTopics(r.cfg.MQTT.TopicPrefix, r.mCfg.ID)
	r.publishDiscovery(driver, topics)

	smsSvc, callSvc, ussdSvc, statusSvc, tariffMgr, diagSvc := r.wireServices(engine, driver, topics)
	statusSvc.SetOnDisconnect(cancel)

	r.mu.Lock()
	r.smsSvc = smsSvc
	r.ussdSvc = ussdSvc
	r.callSvc = callSvc
	r.tariffMgr = tariffMgr
	r.mu.Unlock()

	go r.urcLoop(childCtx, engine, smsSvc, callSvc, ussdSvc)
	r.subscribeMQTT(topics, smsSvc, callSvc, ussdSvc, tariffMgr, diagSvc, driver)
	go statusSvc.Start(childCtx)
	go r.startBalanceLoop(childCtx, ussdSvc, tariffMgr, topics)

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

	balance := 0.0
	currency := "RUB"
	if r.tariffMgr != nil {
		st := r.tariffMgr.Status()
		balance = st.Balance
		currency = st.Currency
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
	r.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("SMS service not initialized")
	}
	return svc.Send(ctx, SendSMSRequest{To: to, Text: text})
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
	return resp.Message, nil
}

func (r *ModemRunner) updateHealth(h ModemHealth) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastHealth = h
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

func (r *ModemRunner) publishDiscovery(driver modem.Driver, topics *mqtt.Topics) {
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

