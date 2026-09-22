// Package main is the entry point for the GSM2MQTT gateway service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/legoser/gsm2mqtt/internal/api"
	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/services"
	"github.com/legoser/gsm2mqtt/internal/transport"
)

// version and buildTime are set at compile time via ldflags.
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	os.Exit(run())
}

// run executes the application and returns an exit code.
func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting gsm2mqtt",
		slog.String("version", version),
		slog.String("build_time", buildTime),
	)

	cfgPath := configPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("failed to load config", slog.String("path", cfgPath), slog.String("error", err.Error()))
		return 1
	}

	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))
	slog.SetDefault(logger)
	logger.Info("config loaded", slog.String("path", cfgPath), slog.Int("modems", len(cfg.Modems)))

	ctx, cancel := setupSignalContext(logger)
	defer cancel()

	return startGateway(ctx, cfg, logger)
}

func startGateway(ctx context.Context, cfg *config.Config, logger *slog.Logger) int {
	mqttClient, err := initMQTT(cfg)
	if err != nil {
		logger.Error("failed to initialize MQTT", slog.String("error", err.Error()))
		return 1
	}
	defer mqttClient.Disconnect(250)

	opener := transport.NewSerialOpener()
	manager := services.NewGatewayManager()
	var wg sync.WaitGroup

	for _, mCfg := range cfg.Modems {
		wg.Add(1)
		runner := services.NewModemRunner(mCfg, cfg, opener, mqttClient)
		manager.Register(runner)

		go func(m config.ModemConfig) {
			defer wg.Done()
			logger.Info("starting modem runner", slog.String("modem", m.ID), slog.String("port", m.Port))
			if err := runner.Run(ctx); err != nil && err != context.Canceled {
				logger.Error("modem runner error", slog.String("modem", m.ID), slog.String("error", err.Error()))
			}
		}(mCfg)
	}

	if cfg.API.Enabled {
		apiServer := api.NewServer(api.ServerConfig{
			Host: cfg.API.Host,
			Port: cfg.API.Port,
		}, manager)
		go func() {
			logger.Info("starting embedded HTTP API", slog.String("host", cfg.API.Host), slog.Int("port", cfg.API.Port))
			if err := apiServer.Start(ctx); err != nil {
				logger.Error("API server error", slog.String("error", err.Error()))
			}
		}()
	}

	<-ctx.Done()
	logger.Info("shutting down runners...")
	wg.Wait()
	_ = mqttClient.Publish(fmt.Sprintf("%s/status", cfg.MQTT.TopicPrefix), 1, true, []byte("offline"))
	logger.Info("gsm2mqtt stopped cleanly")
	return 0
}

func initMQTT(cfg *config.Config) (mqtt.MQTTClient, error) {
	brokerURI := cfg.MQTT.Broker
	if cfg.MQTT.Port > 0 && !strings.Contains(brokerURI, ":") {
		brokerURI = fmt.Sprintf("tcp://%s:%d", brokerURI, cfg.MQTT.Port)
	}

	client, err := mqtt.NewPahoClient(mqtt.ClientConfig{
		Broker:      brokerURI,
		Port:        cfg.MQTT.Port,
		Username:    cfg.MQTT.Username,
		Password:    cfg.MQTT.Password,
		ClientID:    cfg.MQTT.ClientID,
		LWTTopic:    fmt.Sprintf("%s/status", cfg.MQTT.TopicPrefix),
		LWTPayload:  "offline",
		TLSEnabled:  cfg.MQTT.TLS.Enabled,
		InsecureTLS: cfg.MQTT.TLS.InsecureSkipVerify,
	})
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to broker %s: %w", brokerURI, err)
	}

	_ = client.Publish(fmt.Sprintf("%s/status", cfg.MQTT.TopicPrefix), 1, true, []byte("online"))
	return client, nil
}

func setupSignalContext(logger *slog.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))
		cancel()
	}()

	return ctx, cancel
}

func configPath() string {
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return "/etc/gsm2mqtt/gsm2mqtt.yaml"
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Version returns the application version string.
func Version() string {
	return fmt.Sprintf("gsm2mqtt %s (built %s)", version, buildTime)
}
