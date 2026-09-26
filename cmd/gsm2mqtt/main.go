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
	"time"
	_ "time/tzdata"

	"github.com/legoser/gsm2mqtt/internal/api"
	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/pool"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/services"
	"github.com/legoser/gsm2mqtt/internal/system"
	"github.com/legoser/gsm2mqtt/internal/transport"
	verPkg "github.com/legoser/gsm2mqtt/internal/version"
)

// version and buildTime are set at compile time via ldflags.
var (
	version   = verPkg.Version
	buildTime = verPkg.BuildTime
)

func init() {
	if version != "" && version != "dev" {
		verPkg.Version = version
	}
	if buildTime != "" && buildTime != "unknown" {
		verPkg.BuildTime = buildTime
	}
}

func newLogger(level slog.Level, loc *time.Location) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				return slog.String(slog.TimeKey, system.FormatLocalTime(a.Value.Time(), loc))
			}
			return a
		},
	}))
}

func main() {
	os.Exit(run())
}

// run executes the application and returns an exit code.
func run() int {
	if isVersionFlag(os.Args[1:]) {
		fmt.Println(Version())
		return 0
	}

	initialLoc, _ := system.ResolveLocation("")
	if initialLoc != nil {
		time.Local = initialLoc
	}
	logger := newLogger(slog.LevelInfo, initialLoc)
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

	loc, err := system.ResolveLocation(cfg.System.Timezone)
	if err != nil {
		logger.Warn("invalid timezone configured, falling back to system timezone",
			slog.String("timezone", cfg.System.Timezone),
			slog.String("error", err.Error()),
		)
		loc = initialLoc
	}
	if loc != nil {
		time.Local = loc
	}

	logger = newLogger(parseLogLevel(cfg.LogLevel), loc)
	slog.SetDefault(logger)
	logger.Info("config loaded",
		slog.String("path", cfgPath),
		slog.Int("modems", len(cfg.Modems)),
		slog.String("timezone", loc.String()),
	)

	ctx, cancel := setupSignalContext(logger)
	defer cancel()

	return startGateway(ctx, cfg, logger)
}

func startGateway(ctx context.Context, cfg *config.Config, logger *slog.Logger) int {
	recipientsPath := cfg.Security.RecipientsFile
	if recipientsPath == "" {
		recipientsPath = "data/recipients.json"
	}
	recipientsMgr := security.NewRecipientsManager(recipientsPath, cfg.Security.Whitelist)

	manager := services.NewGatewayManager()
	manager.InitRecipients(recipientsMgr)

	onConnect := func(client mqtt.MQTTClient) {
		statusTopic := fmt.Sprintf("%s/status", cfg.MQTT.TopicPrefix)
		if err := client.Publish(statusTopic, byte(cfg.MQTT.QoS), true, []byte("online")); err != nil {
			logger.Warn("failed to publish online status on connect", slog.Any("error", err))
		} else {
			logger.Info("published online status to MQTT", slog.String("topic", statusTopic))
		}
		manager.PublishDiscovery()
		manager.PublishRecipientsState()
	}

	mqttClient, err := initMQTT(cfg, onConnect)
	if err != nil {
		logger.Error("failed to initialize MQTT", slog.String("error", err.Error()))
		return 1
	}
	defer mqttClient.Disconnect(250)

	manager.SetMQTT(mqttClient, &cfg.MQTT)

	modemPool := initPool(ctx, cfg, mqttClient, logger)
	wg := startModems(ctx, cfg, mqttClient, manager, recipientsMgr, modemPool, logger)

	if cfg.API.Enabled {
		startAPIServer(ctx, cfg, manager, logger)
	}

	<-ctx.Done()
	logger.Info("shutting down runners...")
	wg.Wait()
	_ = mqttClient.Publish(fmt.Sprintf("%s/status", cfg.MQTT.TopicPrefix), byte(cfg.MQTT.QoS), true, []byte("offline"))
	logger.Info("gsm2mqtt stopped cleanly")
	return 0
}

func startModems(
	ctx context.Context,
	cfg *config.Config,
	mqttClient mqtt.MQTTClient,
	manager *services.GatewayManager,
	recipientsMgr *security.RecipientsManager,
	modemPool *pool.Pool,
	logger *slog.Logger,
) *sync.WaitGroup {
	connector := modem.NewConnector(transport.NewSerialOpener())
	var wg sync.WaitGroup

	for i, mCfg := range cfg.Modems {
		wg.Add(1)
		runner := services.NewModemRunner(mCfg, cfg, connector, mqttClient)
		runner.SetSlotIndex(i + 1)
		runner.SetRecipientsManager(recipientsMgr)
		manager.Register(runner)
		if modemPool != nil {
			modemPool.Register(runner)
		}

		go func(m config.ModemConfig) {
			defer wg.Done()
			logger.Info("starting modem runner", slog.String("modem", m.ID), slog.String("port", m.Port))
			if err := runner.Run(ctx); err != nil && err != context.Canceled {
				logger.Error("modem runner error", slog.String("modem", m.ID), slog.String("error", err.Error()))
			}
		}(mCfg)
	}
	return &wg
}

func startAPIServer(ctx context.Context, cfg *config.Config, manager *services.GatewayManager, logger *slog.Logger) {
	apiServer := api.NewServer(api.ServerConfig{
		Host:  cfg.API.Host,
		Port:  cfg.API.Port,
		Token: cfg.API.Token,
	}, manager)
	go func() {
		logger.Info("starting embedded HTTP API", slog.String("host", cfg.API.Host), slog.Int("port", cfg.API.Port))
		if err := apiServer.Start(ctx); err != nil {
			logger.Error("API server error", slog.String("error", err.Error()))
		}
	}()
}

func initMQTT(cfg *config.Config, onConnect func(client mqtt.MQTTClient)) (mqtt.MQTTClient, error) {
	brokerURI := cfg.MQTT.Broker
	if cfg.MQTT.Port > 0 && !strings.Contains(brokerURI, ":") {
		scheme := "tcp"
		if cfg.MQTT.TLS.Enabled {
			scheme = "ssl"
		}
		brokerURI = fmt.Sprintf("%s://%s:%d", scheme, brokerURI, cfg.MQTT.Port)
	}

	client, err := mqtt.NewClient(mqtt.ClientConfig{
		Broker:               brokerURI,
		Port:                 cfg.MQTT.Port,
		Username:             cfg.MQTT.Username,
		Password:             cfg.MQTT.Password,
		ClientID:             cfg.MQTT.ClientID,
		QoS:                  byte(cfg.MQTT.QoS),
		CleanSession:         cfg.MQTT.CleanSession,
		KeepAlive:            cfg.MQTT.KeepAlive,
		ConnectTimeout:       cfg.MQTT.ConnectTimeout,
		AutoReconnect:        cfg.MQTT.AutoReconnect,
		MaxReconnectInterval: cfg.MQTT.MaxReconnectInterval,
		LWTTopic:             fmt.Sprintf("%s/status", cfg.MQTT.TopicPrefix),
		LWTPayload:           "offline",
		LWTQoS:               byte(cfg.MQTT.QoS),
		LWTRetained:          true,
		TLSEnabled:           cfg.MQTT.TLS.Enabled,
		InsecureTLS:          cfg.MQTT.TLS.InsecureSkipVerify,
		OnConnect:            onConnect,
	})
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to broker %s: %w", brokerURI, err)
	}

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
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	if env := os.Getenv("GSM2MQTT_CONFIG"); env != "" {
		return env
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		return env
	}
	candidates := []string{
		"configs/gsm2mqtt.yaml",
		"gsm2mqtt.yaml",
		"/etc/gsm2mqtt/gsm2mqtt.yaml",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
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

// isVersionFlag reports whether args request version output.
// It matches exact --version or -v tokens only.
func isVersionFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			return true
		}
	}
	return false
}
