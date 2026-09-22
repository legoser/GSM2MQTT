// Package main is the entry point for the GSM2MQTT gateway service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/legoser/gsm2mqtt/internal/config"
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
// Separated from main() for testability.
func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting gsm2mqtt",
		slog.String("version", version),
		slog.String("build_time", buildTime),
	)

	// Load configuration
	cfgPath := configPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("failed to load config",
			slog.String("path", cfgPath),
			slog.String("error", err.Error()),
		)
		return 1
	}

	// Update log level from config
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))
	slog.SetDefault(logger)

	logger.Info("config loaded",
		slog.String("path", cfgPath),
		slog.Int("modems", len(cfg.Modems)),
	)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))
		cancel()
	}()

	// TODO: Phase 1 — Initialize modem manager, MQTT client, services
	_ = ctx

	logger.Info("gsm2mqtt stopped")
	return 0
}

// configPath returns the configuration file path from args or default.
func configPath() string {
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return "/etc/gsm2mqtt/gsm2mqtt.yaml"
}

// parseLogLevel converts a string log level to slog.Level.
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
