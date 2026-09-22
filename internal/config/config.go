// Package config provides configuration structures and loading
// for the GSM2MQTT gateway service.
package config

import "time"

// Config is the root configuration structure for GSM2MQTT.
type Config struct {
	// LogLevel controls the logging verbosity: debug, info, warn, error.
	LogLevel string `yaml:"log_level"`

	// MQTT holds the MQTT broker connection settings.
	MQTT MQTTConfig `yaml:"mqtt"`

	// Modems is the list of configured GSM modems.
	Modems []ModemConfig `yaml:"modems"`

	// Security holds security-related settings.
	Security SecurityConfig `yaml:"security"`

	// SMS holds SMS encoding and delivery settings.
	SMS SMSConfig `yaml:"sms"`

	// Status holds modem status monitoring settings.
	Status StatusConfig `yaml:"status"`
}

// MQTTConfig holds MQTT broker connection parameters.
type MQTTConfig struct {
	Broker          string    `yaml:"broker"`
	Port            int       `yaml:"port"`
	Username        string    `yaml:"username"`
	Password        string    `yaml:"password"`
	ClientID        string    `yaml:"client_id"`
	TopicPrefix     string    `yaml:"topic_prefix"`
	Discovery       bool      `yaml:"discovery"`
	DiscoveryPrefix string    `yaml:"discovery_prefix"`
	TLS             TLSConfig `yaml:"tls"`
}

// TLSConfig holds TLS/SSL settings for MQTT connection.
type TLSConfig struct {
	Enabled            bool   `yaml:"enabled"`
	CACert             string `yaml:"ca_cert"`
	ClientCert         string `yaml:"client_cert"`
	ClientKey          string `yaml:"client_key"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// ModemConfig holds settings for a single GSM modem.
type ModemConfig struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Port        string `yaml:"port"`
	BaudRate    int    `yaml:"baud_rate"`
	Type        string `yaml:"type"`
	PIN         string `yaml:"pin"`
	DataBits    int    `yaml:"data_bits"`
	StopBits    int    `yaml:"stop_bits"`
	Parity      string `yaml:"parity"`
	FlowControl string `yaml:"flow_control"`
}

// SecurityConfig holds security-related settings.
type SecurityConfig struct {
	IncomingFilter    string          `yaml:"incoming_filter"`
	Whitelist         []string        `yaml:"whitelist"`
	Blacklist         []string        `yaml:"blacklist"`
	RateLimit         RateLimitConfig `yaml:"rate_limit"`
	AllowRawAT        bool            `yaml:"allow_raw_at"`
	BlockedATCommands []string        `yaml:"blocked_at_commands"`
}

// RateLimitConfig holds rate limiting settings for outgoing SMS.
type RateLimitConfig struct {
	Enabled                bool `yaml:"enabled"`
	MaxSMSPerMinute        int  `yaml:"max_sms_per_minute"`
	MaxSMSPerHour          int  `yaml:"max_sms_per_hour"`
	MaxSMSPerDay           int  `yaml:"max_sms_per_day"`
	MaxSMSPerNumberPerHour int  `yaml:"max_sms_per_number_per_hour"`
	CooldownMinutes        int  `yaml:"cooldown_minutes"`
}

// SMSConfig holds SMS encoding and delivery settings.
type SMSConfig struct {
	Encoding       string               `yaml:"encoding"`
	LongMessage    string               `yaml:"long_message"`
	MaxSegments    int                  `yaml:"max_segments"`
	ReportEncoding bool                 `yaml:"report_encoding"`
	DeliveryReport DeliveryReportConfig `yaml:"delivery_report"`
}

// DeliveryReportConfig holds SMS delivery report settings.
type DeliveryReportConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Timeout        time.Duration `yaml:"timeout"`
	PublishPending bool          `yaml:"publish_pending"`
}

// StatusConfig holds modem status monitoring settings.
type StatusConfig struct {
	Interval       time.Duration `yaml:"interval"`
	SignalInterval time.Duration `yaml:"signal_interval"`
}
