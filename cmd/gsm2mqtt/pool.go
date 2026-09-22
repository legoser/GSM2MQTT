package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/metrics"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/pool"
)

func initPool(
	ctx context.Context,
	cfg *config.Config,
	mqttClient mqtt.MQTTClient,
	logger *slog.Logger,
) *pool.Pool {
	if !cfg.Pool.Enabled && len(cfg.Modems) <= 1 {
		return nil
	}

	strat := cfg.Pool.Strategy
	if strat == "" {
		strat = "round-robin"
	}

	p, err := pool.New(pool.Config{
		Strategy:     pool.Strategy(strat),
		DefaultModem: cfg.Pool.DefaultModem,
	})
	if err != nil {
		logger.Error("failed to create modem pool", slog.Any("error", err))
		return nil
	}

	logger.Info("initialized modem pool", slog.String("strategy", strat))
	bindPoolMQTT(ctx, p, cfg, mqttClient, logger)
	return p
}

func bindPoolMQTT(
	ctx context.Context,
	p *pool.Pool,
	cfg *config.Config,
	mqttClient mqtt.MQTTClient,
	logger *slog.Logger,
) {
	topics := mqtt.NewTopics(cfg.MQTT.TopicPrefix, "")

	_ = mqttClient.Subscribe(topics.PoolSMSSend(), 1, func(_ string, payload []byte) {
		var req struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(payload, &req); err == nil {
			_, err := p.SendSMS(ctx, req.To, req.Text)
			if err != nil {
				logger.Error("pool sms send failure", slog.Any("error", err))
			}
		}
	})

	_ = mqttClient.Subscribe(topics.PoolCallDial(), 1, func(_ string, payload []byte) {
		var req struct {
			Number string `json:"number"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Number != "" {
			if err := p.Dial(ctx, req.Number); err != nil {
				logger.Error("pool call dial failure", slog.Any("error", err))
			}
		}
	})

	_ = mqttClient.Subscribe(topics.PoolCallHangup(), 1, func(_ string, _ []byte) {
		_ = p.Hangup(ctx)
	})

	_ = mqttClient.Subscribe(topics.PoolUSSDSend(), 1, func(_ string, payload []byte) {
		var req struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(payload, &req); err == nil && req.Code != "" {
			_, _ = p.SendUSSD(ctx, req.Code)
		}
	})

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				st := p.Status()
				metrics.DefaultRegistry.SetGauge("gsm2mqtt_pool_modems_total", nil, float64(st.TotalModems))
				metrics.DefaultRegistry.SetGauge("gsm2mqtt_pool_modems_ready", nil, float64(st.ReadyModems))
				payload, _ := json.Marshal(st)
				_ = mqttClient.Publish(topics.PoolStatus(), 1, false, payload)
			}
		}
	}()
}
