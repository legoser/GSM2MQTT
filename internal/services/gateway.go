package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
	"github.com/legoser/gsm2mqtt/internal/modem/drivers"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/sms"
	"github.com/legoser/gsm2mqtt/internal/transport"
	"github.com/legoser/gsm2mqtt/internal/ussd"
)

// ModemRunner manages the complete lifecycle, AT engine, and MQTT bridging for a single modem.
type ModemRunner struct {
	mCfg       config.ModemConfig
	cfg        *config.Config
	opener     transport.Opener
	mqttClient mqtt.MQTTClient
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

// Run starts serial communication, initializes modem drivers, and binds MQTT handlers.
func (r *ModemRunner) Run(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	port, err := r.opener.Open(transport.PortConfig{
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
	defer port.Close()

	engine := at.NewEngine(port)
	go func() {
		_ = engine.Start(childCtx)
	}()

	driver := r.createDriver(engine)
	if err := driver.Init(childCtx); err != nil {
		slog.Warn("driver init completed with warning", slog.String("modem", r.mCfg.ID), slog.Any("error", err))
	}

	topics := mqtt.NewTopics(r.cfg.MQTT.TopicPrefix, r.mCfg.ID)
	r.publishDiscovery(driver, topics)

	smsSvc, callSvc, ussdSvc, statusSvc := r.wireServices(engine, driver, topics)

	go r.urcLoop(childCtx, engine, smsSvc, callSvc, ussdSvc)
	r.subscribeMQTT(topics, smsSvc, callSvc, ussdSvc, driver)
	go statusSvc.Start(childCtx)

	<-ctx.Done()
	return ctx.Err()
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

func (r *ModemRunner) wireServices(
	engine *at.Engine,
	driver modem.Driver,
	topics *mqtt.Topics,
) (*SMSService, *CallService, *USSDService, *StatusService) {
	filter := security.NewFilter(r.cfg.Security.IncomingFilter, r.cfg.Security.Whitelist, r.cfg.Security.Blacklist)
	limiter := security.NewRateLimiterWithConfig(security.RateLimiterConfig{
		Enabled:             r.cfg.Security.RateLimit.Enabled,
		MaxPerMinute:        r.cfg.Security.RateLimit.MaxSMSPerMinute,
		MaxPerHour:          r.cfg.Security.RateLimit.MaxSMSPerHour,
		MaxPerDay:           r.cfg.Security.RateLimit.MaxSMSPerDay,
		MaxPerNumberPerHour: r.cfg.Security.RateLimit.MaxSMSPerNumberPerHour,
		Cooldown:            time.Duration(r.cfg.Security.RateLimit.CooldownMinutes) * time.Minute,
	})

	tracker := sms.NewTracker(r.cfg.SMS.DeliveryReport.Timeout, func(e sms.DeliveryEvent) {
		payload, _ := json.Marshal(e)
		_ = r.mqttClient.Publish(topics.SMSStatus(), 1, false, payload)
	})

	assembler := sms.NewAssembler(24 * time.Hour)
	sender := &atPDUSender{engine: engine}

	smsSvc := NewSMSService(SMSServiceConfig{
		ModemID:        r.mCfg.ID,
		Transliterate:  r.cfg.SMS.Encoding == "translit",
		DeliveryReport: r.cfg.SMS.DeliveryReport.Enabled,
	}, sender, filter, limiter, tracker, assembler, func(msg *sms.AssembledSMS) {
		payload, _ := json.Marshal(msg)
		_ = r.mqttClient.Publish(topics.SMSReceived(), 1, false, payload)
	})

	callSvc := NewCallService(r.mCfg.ID, driver, func(e CallEvent) {
		payload, _ := json.Marshal(e)
		if e.Type == "incoming" || e.Type == "ended" {
			_ = r.mqttClient.Publish(topics.CallIncoming(), 1, false, payload)
		} else if e.Type == "dtmf" {
			_ = r.mqttClient.Publish(topics.CallDTMF(), 1, false, payload)
		}
	})

	ussdSvc := NewUSSDService(r.mCfg.ID, driver, func(resp *ussd.Response) {
		payload, _ := json.Marshal(resp)
		_ = r.mqttClient.Publish(topics.USSDResponse(), 1, false, payload)
	})

	statusSvc := NewStatusService(StatusServiceConfig{
		ModemID:  r.mCfg.ID,
		Interval: r.cfg.Status.Interval,
	}, driver, func(rssi, dbm int) {
		payload := fmt.Sprintf(`{"rssi":%d,"dbm":%d}`, rssi, dbm)
		_ = r.mqttClient.Publish(topics.SignalStrength(), 1, false, []byte(payload))
	}, func(h ModemHealth) {
		payload, _ := json.Marshal(h)
		_ = r.mqttClient.Publish(topics.Health(), 1, false, payload)
	})

	return smsSvc, callSvc, ussdSvc, statusSvc
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

func (r *ModemRunner) subscribeMQTT(
	topics *mqtt.Topics,
	smsSvc *SMSService,
	callSvc *CallService,
	ussdSvc *USSDService,
	driver modem.Driver,
) {
	_ = r.mqttClient.Subscribe(topics.SMSSend(), 1, func(_ string, payload []byte) {
		var req SendSMSRequest
		if err := json.Unmarshal(payload, &req); err == nil {
			_, _ = smsSvc.Send(context.Background(), req)
		}
	})

	_ = r.mqttClient.Subscribe(topics.CallDial(), 1, func(_ string, payload []byte) {
		var req struct {
			Number string `json:"number"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Number != "" {
			_ = callSvc.Dial(context.Background(), req.Number)
		}
	})

	_ = r.mqttClient.Subscribe(topics.CallHangup(), 1, func(_ string, _ []byte) {
		_ = callSvc.Hangup(context.Background())
	})

	_ = r.mqttClient.Subscribe(topics.USSDSend(), 1, func(_ string, payload []byte) {
		var req struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Code != "" {
			_, _ = ussdSvc.Send(context.Background(), req.Code)
		}
	})
}

type atPDUSender struct {
	engine *at.Engine
}

func (s *atPDUSender) SendPDU(cmdLength int, pduHex string) (byte, error) {
	cmd := fmt.Sprintf("AT+CMGS=%d\r%s\x1A", cmdLength, pduHex)
	resp, err := s.engine.Send(cmd, 15*time.Second)
	if err != nil {
		return 0, err
	}
	if resp.Error {
		return 0, fmt.Errorf("PDU send returned error: %v", resp.Lines)
	}
	for _, line := range resp.Lines {
		if strings.HasPrefix(line, "+CMGS:") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				ref, err := strconv.Atoi(parts[1])
				if err == nil {
					return byte(ref), nil
				}
			}
		}
	}
	return 0, nil
}
