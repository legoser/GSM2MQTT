package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHierarchicalPriority(t *testing.T) {
	// OS Env > .env > YAML > Defaults
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	envFile := filepath.Join(tmpDir, ".env")

	yamlContent := `
mqtt:
  broker: "yaml-broker"
  port: 1883
  username: "yaml-user"
`
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	envContent := `
GSM2MQTT_MQTT_BROKER=dotenv-broker
GSM2MQTT_MQTT_USERNAME=dotenv-user
`
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// 1. Without OS env: .env should override YAML
	cfg1, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg1.MQTT.Broker != "dotenv-broker" {
		t.Errorf("expected .env broker 'dotenv-broker', got %q", cfg1.MQTT.Broker)
	}
	if cfg1.MQTT.Username != "dotenv-user" {
		t.Errorf("expected .env username 'dotenv-user', got %q", cfg1.MQTT.Username)
	}

	// 2. With OS env: OS env should override .env
	t.Setenv("GSM2MQTT_MQTT_BROKER", "os-broker")
	cfg2, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load with OS env failed: %v", err)
	}
	if cfg2.MQTT.Broker != "os-broker" {
		t.Errorf("expected OS env broker 'os-broker', got %q", cfg2.MQTT.Broker)
	}
	// Username was not in OS env, so it should still come from .env
	if cfg2.MQTT.Username != "dotenv-user" {
		t.Errorf("expected .env username 'dotenv-user', got %q", cfg2.MQTT.Username)
	}
}

func TestAllHierarchicalOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write empty config: %v", err)
	}

	t.Setenv("GSM2MQTT_LOG_LEVEL", "debug")
	// MQTT
	t.Setenv("GSM2MQTT_MQTT_BROKER", "192.168.57.254")
	t.Setenv("GSM2MQTT_MQTT_PORT", "8883")
	t.Setenv("GSM2MQTT_MQTT_USERNAME", "mqtt")
	t.Setenv("GSM2MQTT_MQTT_PASSWORD", "65432!")
	t.Setenv("GSM2MQTT_MQTT_TOPIC_PREFIX", "custom_topic")
	t.Setenv("GSM2MQTT_MQTT_DISCOVERY", "false")
	t.Setenv("GSM2MQTT_MQTT_DISCOVERY_PREFIX", "custom_ha")
	t.Setenv("GSM2MQTT_MQTT_TLS_ENABLED", "true")
	t.Setenv("GSM2MQTT_MQTT_TLS_CA_CERT", "/path/to/ca.crt")
	t.Setenv("GSM2MQTT_MQTT_TLS_CLIENT_CERT", "/path/to/client.crt")
	t.Setenv("GSM2MQTT_MQTT_TLS_CLIENT_KEY", "/path/to/client.key")
	t.Setenv("GSM2MQTT_MQTT_TLS_INSECURE_SKIP_VERIFY", "true")

	// Modem (should auto-create modems[0] if empty)
	t.Setenv("MODEM_DEVICE", "/dev/ttyUSB99")
	t.Setenv("GSM2MQTT_MODEM_ID", "modem_test")
	t.Setenv("GSM2MQTT_MODEM_NAME", "Test Modem")
	t.Setenv("GSM2MQTT_MODEM_TYPE", "neoway")
	t.Setenv("GSM2MQTT_MODEM_BAUD_RATE", "115200")
	t.Setenv("GSM2MQTT_MODEM_PIN", "1234")

	// Security
	t.Setenv("GSM2MQTT_SECURITY_INCOMING_FILTER", "whitelist")
	t.Setenv("GSM2MQTT_SECURITY_ALLOW_RAW_AT", "true")
	t.Setenv("GSM2MQTT_RATE_LIMIT_ENABLED", "false")
	t.Setenv("GSM2MQTT_MAX_SMS_PER_MINUTE", "10")
	t.Setenv("GSM2MQTT_MAX_SMS_PER_HOUR", "50")
	t.Setenv("GSM2MQTT_MAX_SMS_PER_DAY", "200")
	t.Setenv("GSM2MQTT_MAX_SMS_PER_NUMBER_PER_HOUR", "5")
	t.Setenv("GSM2MQTT_RATE_LIMIT_COOLDOWN_MINUTES", "15")

	// SMS
	t.Setenv("GSM2MQTT_SMS_ENCODING", "ucs2")
	t.Setenv("GSM2MQTT_SMS_LONG_MESSAGE", "truncate")
	t.Setenv("GSM2MQTT_SMS_MAX_SEGMENTS", "8")
	t.Setenv("GSM2MQTT_SMS_REPORT_ENCODING", "false")
	t.Setenv("GSM2MQTT_SMS_DELIVERY_REPORT_ENABLED", "false")
	t.Setenv("GSM2MQTT_SMS_DELIVERY_REPORT_TIMEOUT", "10m")
	t.Setenv("GSM2MQTT_SMS_DELIVERY_REPORT_PUBLISH_PENDING", "false")

	// Status
	t.Setenv("GSM2MQTT_STATUS_INTERVAL", "45s")
	t.Setenv("GSM2MQTT_STATUS_SIGNAL_INTERVAL", "90s")

	// Tariff
	t.Setenv("GSM2MQTT_TARIFF_ENABLED", "true")
	t.Setenv("GSM2MQTT_TARIFF_OPERATOR_PRESET", "megafon")
	t.Setenv("GSM2MQTT_TARIFF_BALANCE_USSD", "*105#")
	t.Setenv("GSM2MQTT_TARIFF_BALANCE_REGEX", `(\d+[\.,]\d+)`)
	t.Setenv("GSM2MQTT_TARIFF_AUTO_CHECK_ON_ERROR", "false")
	t.Setenv("GSM2MQTT_TARIFF_CHECK_INTERVAL", "12h")
	t.Setenv("GSM2MQTT_TARIFF_MIN_BALANCE_ALERT", "25.5")
	t.Setenv("GSM2MQTT_TARIFF_SMS_LIMIT", "250")
	t.Setenv("GSM2MQTT_TARIFF_CALL_MINUTES_LIMIT", "300")
	t.Setenv("GSM2MQTT_TARIFF_DATA_TRAFFIC_LIMIT_MB", "5000")
	t.Setenv("GSM2MQTT_TARIFF_RESET_DAY_OF_MONTH", "15")
	t.Setenv("GSM2MQTT_TARIFF_STORAGE_DIR", "/custom/data")

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Assertions
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want 'debug'", cfg.LogLevel)
	}
	if cfg.MQTT.Broker != "192.168.57.254" {
		t.Errorf("MQTT.Broker = %q, want '192.168.57.254'", cfg.MQTT.Broker)
	}
	if cfg.MQTT.Port != 8883 {
		t.Errorf("MQTT.Port = %d, want 8883", cfg.MQTT.Port)
	}
	if cfg.MQTT.Username != "mqtt" {
		t.Errorf("MQTT.Username = %q, want 'mqtt'", cfg.MQTT.Username)
	}
	if cfg.MQTT.Password != "65432!" {
		t.Errorf("MQTT.Password = %q, want '65432!'", cfg.MQTT.Password)
	}
	if cfg.MQTT.TopicPrefix != "custom_topic" {
		t.Errorf("MQTT.TopicPrefix = %q, want 'custom_topic'", cfg.MQTT.TopicPrefix)
	}
	if cfg.MQTT.Discovery != false {
		t.Errorf("MQTT.Discovery = %v, want false", cfg.MQTT.Discovery)
	}
	if cfg.MQTT.DiscoveryPrefix != "custom_ha" {
		t.Errorf("MQTT.DiscoveryPrefix = %q, want 'custom_ha'", cfg.MQTT.DiscoveryPrefix)
	}
	if !cfg.MQTT.TLS.Enabled {
		t.Errorf("MQTT.TLS.Enabled = false, want true")
	}
	if cfg.MQTT.TLS.CACert != "/path/to/ca.crt" {
		t.Errorf("MQTT.TLS.CACert = %q, want '/path/to/ca.crt'", cfg.MQTT.TLS.CACert)
	}
	if !cfg.MQTT.TLS.InsecureSkipVerify {
		t.Errorf("MQTT.TLS.InsecureSkipVerify = false, want true")
	}

	// Modem
	if len(cfg.Modems) != 1 {
		t.Fatalf("len(cfg.Modems) = %d, want 1", len(cfg.Modems))
	}
	m := cfg.Modems[0]
	if m.Port != "/dev/ttyUSB99" {
		t.Errorf("Modem.Port = %q, want '/dev/ttyUSB99'", m.Port)
	}
	if m.ID != "modem_test" {
		t.Errorf("Modem.ID = %q, want 'modem_test'", m.ID)
	}
	if m.Name != "Test Modem" {
		t.Errorf("Modem.Name = %q, want 'Test Modem'", m.Name)
	}
	if m.Type != "neoway" {
		t.Errorf("Modem.Type = %q, want 'neoway'", m.Type)
	}
	if m.BaudRate != 115200 {
		t.Errorf("Modem.BaudRate = %d, want 115200", m.BaudRate)
	}
	if m.PIN != "1234" {
		t.Errorf("Modem.PIN = %q, want '1234'", m.PIN)
	}

	// Security
	if cfg.Security.IncomingFilter != "whitelist" {
		t.Errorf("IncomingFilter = %q, want 'whitelist'", cfg.Security.IncomingFilter)
	}
	if !cfg.Security.AllowRawAT {
		t.Errorf("AllowRawAT = false, want true")
	}
	if cfg.Security.RateLimit.Enabled != false {
		t.Errorf("RateLimit.Enabled = true, want false")
	}
	if cfg.Security.RateLimit.MaxSMSPerMinute != 10 {
		t.Errorf("MaxSMSPerMinute = %d, want 10", cfg.Security.RateLimit.MaxSMSPerMinute)
	}
	if cfg.Security.RateLimit.MaxSMSPerHour != 50 {
		t.Errorf("MaxSMSPerHour = %d, want 50", cfg.Security.RateLimit.MaxSMSPerHour)
	}
	if cfg.Security.RateLimit.MaxSMSPerDay != 200 {
		t.Errorf("MaxSMSPerDay = %d, want 200", cfg.Security.RateLimit.MaxSMSPerDay)
	}
	if cfg.Security.RateLimit.MaxSMSPerNumberPerHour != 5 {
		t.Errorf("MaxSMSPerNumberPerHour = %d, want 5", cfg.Security.RateLimit.MaxSMSPerNumberPerHour)
	}
	if cfg.Security.RateLimit.CooldownMinutes != 15 {
		t.Errorf("CooldownMinutes = %d, want 15", cfg.Security.RateLimit.CooldownMinutes)
	}

	// SMS
	if cfg.SMS.Encoding != "ucs2" {
		t.Errorf("SMS.Encoding = %q, want 'ucs2'", cfg.SMS.Encoding)
	}
	if cfg.SMS.LongMessage != "truncate" {
		t.Errorf("SMS.LongMessage = %q, want 'truncate'", cfg.SMS.LongMessage)
	}
	if cfg.SMS.MaxSegments != 8 {
		t.Errorf("SMS.MaxSegments = %d, want 8", cfg.SMS.MaxSegments)
	}
	if cfg.SMS.ReportEncoding != false {
		t.Errorf("SMS.ReportEncoding = true, want false")
	}
	if cfg.SMS.DeliveryReport.Enabled != false {
		t.Errorf("DeliveryReport.Enabled = true, want false")
	}
	if cfg.SMS.DeliveryReport.Timeout != 10*time.Minute {
		t.Errorf("DeliveryReport.Timeout = %v, want 10m", cfg.SMS.DeliveryReport.Timeout)
	}

	// Status
	if cfg.Status.Interval != 45*time.Second {
		t.Errorf("Status.Interval = %v, want 45s", cfg.Status.Interval)
	}
	if cfg.Status.SignalInterval != 90*time.Second {
		t.Errorf("Status.SignalInterval = %v, want 90s", cfg.Status.SignalInterval)
	}

	// Tariff
	if cfg.Tariff.OperatorPreset != "megafon" {
		t.Errorf("Tariff.OperatorPreset = %q, want 'megafon'", cfg.Tariff.OperatorPreset)
	}
	if cfg.Tariff.BalanceUSSD != "*105#" {
		t.Errorf("Tariff.BalanceUSSD = %q, want '*105#'", cfg.Tariff.BalanceUSSD)
	}
	if cfg.Tariff.AutoCheckOnError != false {
		t.Errorf("Tariff.AutoCheckOnError = true, want false")
	}
	if cfg.Tariff.CheckInterval != 12*time.Hour {
		t.Errorf("Tariff.CheckInterval = %v, want 12h", cfg.Tariff.CheckInterval)
	}
	if cfg.Tariff.MinBalanceAlert != 25.5 {
		t.Errorf("Tariff.MinBalanceAlert = %f, want 25.5", cfg.Tariff.MinBalanceAlert)
	}
	if cfg.Tariff.SMSLimit != 250 {
		t.Errorf("Tariff.SMSLimit = %d, want 250", cfg.Tariff.SMSLimit)
	}
	if cfg.Tariff.CallMinutesLimit != 300.0 {
		t.Errorf("Tariff.CallMinutesLimit = %f, want 300.0", cfg.Tariff.CallMinutesLimit)
	}
	if cfg.Tariff.DataTrafficLimitMB != 5000 {
		t.Errorf("Tariff.DataTrafficLimitMB = %d, want 5000", cfg.Tariff.DataTrafficLimitMB)
	}
	if cfg.Tariff.ResetDayOfMonth != 15 {
		t.Errorf("Tariff.ResetDayOfMonth = %d, want 15", cfg.Tariff.ResetDayOfMonth)
	}
	if cfg.Tariff.StorageDir != "/custom/data" {
		t.Errorf("Tariff.StorageDir = %q, want '/custom/data'", cfg.Tariff.StorageDir)
	}
}

func TestDotEnvParsingVariations(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `
# Comment line
export GSM2MQTT_MQTT_BROKER="192.168.1.10"
MQTT_PORT=1883 # inline comment
GSM2MQTT_MQTT_PASSWORD='secret!pass#word'
EMPTY_VAL=
SPACED_KEY = spaced_val
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	m := parseEnvFile(envPath)
	if m["GSM2MQTT_MQTT_BROKER"] != "192.168.1.10" {
		t.Errorf("GSM2MQTT_MQTT_BROKER = %q, want '192.168.1.10'", m["GSM2MQTT_MQTT_BROKER"])
	}
	if m["MQTT_PORT"] != "1883" {
		t.Errorf("MQTT_PORT = %q, want '1883'", m["MQTT_PORT"])
	}
	if m["GSM2MQTT_MQTT_PASSWORD"] != "secret!pass#word" {
		t.Errorf("GSM2MQTT_MQTT_PASSWORD = %q, want 'secret!pass#word'", m["GSM2MQTT_MQTT_PASSWORD"])
	}
	if m["SPACED_KEY"] != "spaced_val" {
		t.Errorf("SPACED_KEY = %q, want 'spaced_val'", m["SPACED_KEY"])
	}
}
