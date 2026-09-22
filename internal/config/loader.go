package config

import (
	"fmt"
	"os"
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

	applyEnvOverrides(cfg)

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
// Pattern: GSM2MQTT_<SECTION>_<FIELD> in uppercase.
func applyEnvOverrides(cfg *Config) {
	overrides := map[string]func(string){
		"GSM2MQTT_LOG_LEVEL":      func(v string) { cfg.LogLevel = v },
		"GSM2MQTT_MQTT_BROKER":    func(v string) { cfg.MQTT.Broker = v },
		"GSM2MQTT_MQTT_PORT":      func(v string) { cfg.MQTT.Port = atoi(v, cfg.MQTT.Port) },
		"GSM2MQTT_MQTT_USERNAME":  func(v string) { cfg.MQTT.Username = v },
		"GSM2MQTT_MQTT_PASSWORD":  func(v string) { cfg.MQTT.Password = v },
		"GSM2MQTT_MQTT_CLIENT_ID": func(v string) { cfg.MQTT.ClientID = v },
	}

	for env, setter := range overrides {
		if v := os.Getenv(env); v != "" {
			setter(v)
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
