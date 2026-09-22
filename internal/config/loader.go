package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Load reads the configuration from a YAML file and applies
// environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	if err := loadFromFile(cfg, path); err != nil {
		return nil, fmt.Errorf("loading config file: %w", err)
	}

	dotEnv := loadDotEnv(filepath.Dir(path))
	applyHierarchicalOverrides(cfg, dotEnv)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// Defaults returns a Config with sensible default values.
func Defaults() *Config {
	return &Config{
		LogLevel: "info",
		MQTT: MQTTConfig{
			Broker:          "localhost",
			Port:            1883,
			ClientID:        "gsm2mqtt",
			TopicPrefix:     "gsm2mqtt",
			Discovery:       true,
			DiscoveryPrefix: "homeassistant",
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
			BlockedATCommands: []string{
				"AT+CFUN=0",
				"AT+CPIN",
				"ATD",
				"AT&F",
			},
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
			ResetDayOfMonth:  1,
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
			// No config file — use defaults
			return nil
		}
		return fmt.Errorf("reading file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func applyEnvOverrides(cfg *Config) {
	applyHierarchicalOverrides(cfg, nil)
}

// applyHierarchicalOverrides applies configuration overrides with priority: OS Env > .env > YAML.
func applyHierarchicalOverrides(cfg *Config, dotEnv map[string]string) {
	overrides := []struct {
		keys   []string
		setter func(string)
	}{
		{[]string{"GSM2MQTT_LOG_LEVEL", "LOG_LEVEL"}, func(v string) { cfg.LogLevel = v }},
		{[]string{"GSM2MQTT_MQTT_BROKER", "MQTT_BROKER"}, func(v string) { cfg.MQTT.Broker = v }},
		{[]string{"GSM2MQTT_MQTT_PORT", "MQTT_PORT"}, func(v string) { cfg.MQTT.Port = atoi(v, cfg.MQTT.Port) }},
		{[]string{"GSM2MQTT_MQTT_USERNAME", "MQTT_USERNAME"}, func(v string) { cfg.MQTT.Username = v }},
		{[]string{"GSM2MQTT_MQTT_PASSWORD", "MQTT_PASSWORD"}, func(v string) { cfg.MQTT.Password = v }},
		{[]string{"GSM2MQTT_MQTT_CLIENT_ID", "MQTT_CLIENT_ID"}, func(v string) { cfg.MQTT.ClientID = v }},
		{[]string{"GSM2MQTT_POOL_ENABLED", "POOL_ENABLED"}, func(v string) { cfg.Pool.Enabled = v == "true" || v == "1" }},
		{[]string{"GSM2MQTT_POOL_STRATEGY", "POOL_STRATEGY"}, func(v string) { cfg.Pool.Strategy = v }},
		{[]string{"GSM2MQTT_POOL_DEFAULT_MODEM", "POOL_DEFAULT_MODEM"}, func(v string) { cfg.Pool.DefaultModem = v }},
		{[]string{"GSM2MQTT_API_ENABLED", "API_ENABLED"}, func(v string) { cfg.API.Enabled = v == "true" || v == "1" }},
		{[]string{"GSM2MQTT_API_HOST", "API_HOST"}, func(v string) { cfg.API.Host = v }},
		{[]string{"GSM2MQTT_API_PORT", "API_PORT"}, func(v string) { cfg.API.Port = atoi(v, cfg.API.Port) }},
		{[]string{"GSM2MQTT_MODEM_PORT", "MODEM_DEVICE", "MODEM_PORT"}, func(v string) {
			if len(cfg.Modems) > 0 {
				cfg.Modems[0].Port = v
			}
		}},
	}

	for _, o := range overrides {
		if v := getHierarchicalValue(dotEnv, o.keys...); v != "" {
			o.setter(v)
		}
	}
}

// validate checks the configuration for required fields and valid values.
func validate(cfg *Config) error {
	if cfg.MQTT.Broker == "" {
		return fmt.Errorf("mqtt.broker is required")
	}
	if cfg.MQTT.Port <= 0 || cfg.MQTT.Port > 65535 {
		return fmt.Errorf("mqtt.port must be between 1 and 65535, got %d", cfg.MQTT.Port)
	}
	if cfg.MQTT.TopicPrefix == "" {
		return fmt.Errorf("mqtt.topic_prefix is required")
	}

	validEncodings := map[string]bool{"auto": true, "translit": true, "ucs2": true, "gsm7": true}
	if !validEncodings[cfg.SMS.Encoding] {
		return fmt.Errorf("sms.encoding must be one of: auto, translit, ucs2, gsm7; got %q", cfg.SMS.Encoding)
	}

	validLongMsg := map[string]bool{"split": true, "truncate": true, "reject": true}
	if !validLongMsg[cfg.SMS.LongMessage] {
		return fmt.Errorf("sms.long_message must be one of: split, truncate, reject; got %q", cfg.SMS.LongMessage)
	}

	validFilters := map[string]bool{"all": true, "whitelist": true, "blacklist": true}
	if !validFilters[cfg.Security.IncomingFilter] {
		return fmt.Errorf("security.incoming_filter must be one of: all, whitelist, blacklist; got %q", cfg.Security.IncomingFilter)
	}

	for i, m := range cfg.Modems {
		if m.ID == "" {
			return fmt.Errorf("modems[%d].id is required", i)
		}
		if m.Port == "" {
			return fmt.Errorf("modems[%d].port is required", i)
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
			return fmt.Errorf("pool.strategy must be one of: round-robin, failover, best-signal, operator-match; got %q", cfg.Pool.Strategy)
		}
	}

	return nil
}

// atoi converts a string to int, returning defaultVal on parse error.
func atoi(s string, defaultVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
