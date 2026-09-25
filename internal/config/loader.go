package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Load reads the configuration from a YAML file and applies
// environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	resolvedPath := path
	if resolvedPath == "" {
		resolvedPath = findDefaultConfig()
	}

	if resolvedPath != "" {
		if err := loadFromFile(cfg, resolvedPath); err != nil {
			return nil, fmt.Errorf("invalid config: loading config file: %w", err)
		}
	}

	dotEnv := loadDotEnv(filepath.Dir(resolvedPath))
	applyHierarchicalOverrides(cfg, dotEnv)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: validating config: %w", err)
	}

	return cfg, nil
}

// Defaults returns a Config with sensible default values.
func Defaults() *Config {
	return &Config{
		LogLevel: "info",
		MQTT: MQTTConfig{
			Broker:               "localhost",
			Port:                 1883,
			ClientID:             "gsm2mqtt",
			TopicPrefix:          "gsm2mqtt",
			QoS:                  1,
			CleanSession:         true,
			KeepAlive:            60 * time.Second,
			ConnectTimeout:       10 * time.Second,
			AutoReconnect:        true,
			MaxReconnectInterval: 10 * time.Minute,
			Discovery:            true,
			DiscoveryPrefix:      "homeassistant",
		},
		Security: SecurityConfig{
			IncomingFilter: "all",
			RateLimit: RateLimitConfig{
				Enabled:                true,
				MaxSMSPerMinute:        5,
				MaxSMSPerHour:          30,
				MaxSMSPerDay:           100,
				MaxSMSPerNumberPerHour: 3,
				CooldownMinutes:        10,
			},
			AllowRawAT: false,
			AllowedATCommands: []string{
				"ATI",
				"AT+CSQ",
				"AT+CREG",
				"AT+CGREG",
				"AT+CEREG",
				"AT+COPS",
				"AT+CPIN?",
				"AT+CSCA",
				"AT+CMGF",
				"AT+CNMI",
				"AT+CMEE",
				"AT+CBC",
				"AT+CGMI",
				"AT+CGMM",
				"AT+CGMR",
				"AT+CGSN",
				"AT+CIMI",
				"AT+CCLK",
				"AT+CSMS",
				"AT+CPMS",
				"AT+CMGL",
				"AT+CMGR",
				"AT+CLCC",
				"AT+CLIP",
				"AT+COLP",
				"AT+CFUN?",
				"AT+CVOICE",
				"AT^CVOICE",
				"AT+CSCLK",
			},
			RecipientsFile: "data/recipients.json",
			FallbackCall:   true,
		},
		SMS: SMSConfig{
			Encoding:       "auto",
			LongMessage:    "split",
			MaxSegments:    4,
			ReportEncoding: true,
			DeliveryReport: DeliveryReportConfig{
				Enabled:        true,
				Timeout:        5 * time.Minute,
				PublishPending: true,
			},
		},
		Status: StatusConfig{
			Interval:       30 * time.Second,
			SignalInterval: 60 * time.Second,
		},
		Tariff: TariffConfig{
			Enabled:          true,
			OperatorPreset:   "generic",
			BalanceUSSD:      "*100#",
			AutoCheckOnError: true,
			CheckInterval:    24 * time.Hour,
			MinBalanceAlert:  50.0,
			SMSLimit:         100,
			ResetDayOfMonth:  1,
			StorageDir:       "data",
		},
		API: APIConfig{
			Enabled: false,
			Host:    "127.0.0.1",
			Port:    8080,
		},
		Pool: PoolConfig{
			Enabled:      false,
			Strategy:     "round-robin",
			DefaultModem: "",
		},
	}
}

// loadFromFile reads a YAML file into the config struct.
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if path == "/etc/gsm2mqtt/gsm2mqtt.yaml" {
				if alt := findDefaultConfig(); alt != "" && alt != path {
					return loadFromFile(cfg, alt)
				}
			}
			// No config file — use defaults
			return nil
		}
		return fmt.Errorf("invalid config: reading file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("invalid config: parsing YAML: %w", err)
	}

	return nil
}

// findDefaultConfig searches for a configuration file in standard locations.
func findDefaultConfig() string {
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
	return ""
}

// validate checks the configuration for required fields and valid values.
func validate(cfg *Config) error {
	if cfg.MQTT.Broker == "" {
		return fmt.Errorf("invalid config: mqtt.broker is required")
	}
	if cfg.MQTT.Port <= 0 || cfg.MQTT.Port > 65535 {
		return fmt.Errorf("invalid config: mqtt.port must be between 1 and 65535, got %d", cfg.MQTT.Port)
	}
	if cfg.MQTT.TopicPrefix == "" {
		return fmt.Errorf("invalid config: mqtt.topic_prefix is required")
	}
	if cfg.MQTT.QoS < 0 || cfg.MQTT.QoS > 2 {
		return fmt.Errorf("invalid config: mqtt.qos must be between 0 and 2, got %d", cfg.MQTT.QoS)
	}
	if cfg.MQTT.KeepAlive < 0 {
		return fmt.Errorf("invalid config: mqtt.keep_alive cannot be negative")
	}
	if cfg.MQTT.ConnectTimeout < 0 {
		return fmt.Errorf("invalid config: mqtt.connect_timeout cannot be negative")
	}
	if cfg.MQTT.MaxReconnectInterval < 0 {
		return fmt.Errorf("invalid config: mqtt.max_reconnect_interval cannot be negative")
	}

	validEncodings := map[string]bool{"auto": true, "translit": true, "ucs2": true, "gsm7": true}
	if !validEncodings[cfg.SMS.Encoding] {
		return fmt.Errorf("invalid config: sms.encoding must be one of: auto, translit, ucs2, gsm7; got %q", cfg.SMS.Encoding)
	}

	validLongMsg := map[string]bool{"split": true, "truncate": true, "reject": true}
	if !validLongMsg[cfg.SMS.LongMessage] {
		return fmt.Errorf("invalid config: sms.long_message must be one of: split, truncate, reject; got %q", cfg.SMS.LongMessage)
	}

	validFilters := map[string]bool{"all": true, "whitelist": true, "blacklist": true}
	if !validFilters[cfg.Security.IncomingFilter] {
		return fmt.Errorf("invalid config: security.incoming_filter must be one of: all, whitelist, blacklist; got %q", cfg.Security.IncomingFilter)
	}

	if len(cfg.Security.BlockedATCommands) > 0 {
		return fmt.Errorf("invalid config: security.blocked_at_commands is deprecated, use security.allowed_at_commands instead")
	}

	for i, m := range cfg.Modems {
		if m.ID == "" {
			return fmt.Errorf("invalid config: modems[%d].id is required", i)
		}
		if m.Port == "" {
			return fmt.Errorf("invalid config: modems[%d].port is required", i)
		}
	}

	if cfg.Pool.Enabled && cfg.Pool.Strategy != "" {
		validStrategies := map[string]bool{
			"round-robin":    true,
			"failover":       true,
			"best-signal":    true,
			"operator-match": true,
		}
		if !validStrategies[cfg.Pool.Strategy] {
			return fmt.Errorf("invalid config: pool.strategy must be one of: round-robin, failover, best-signal, operator-match; got %q", cfg.Pool.Strategy)
		}
	}

	return nil
}
