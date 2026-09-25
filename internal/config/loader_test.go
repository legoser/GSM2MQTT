package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got %q", cfg.LogLevel)
	}
	if cfg.MQTT.Broker != "localhost" {
		t.Errorf("expected MQTT.Broker 'localhost', got %q", cfg.MQTT.Broker)
	}
	if cfg.MQTT.Port != 1883 {
		t.Errorf("expected MQTT.Port 1883, got %d", cfg.MQTT.Port)
	}
	if cfg.MQTT.TopicPrefix != "gsm2mqtt" {
		t.Errorf("expected MQTT.TopicPrefix 'gsm2mqtt', got %q", cfg.MQTT.TopicPrefix)
	}
	if cfg.MQTT.QoS != 1 {
		t.Errorf("expected default MQTT.QoS 1, got %d", cfg.MQTT.QoS)
	}
	if !cfg.MQTT.CleanSession {
		t.Errorf("expected default MQTT.CleanSession true")
	}
	if cfg.MQTT.KeepAlive != 60*time.Second {
		t.Errorf("expected default MQTT.KeepAlive 60s, got %v", cfg.MQTT.KeepAlive)
	}
	if cfg.MQTT.ConnectTimeout != 10*time.Second {
		t.Errorf("expected default MQTT.ConnectTimeout 10s, got %v", cfg.MQTT.ConnectTimeout)
	}
	if !cfg.MQTT.AutoReconnect {
		t.Errorf("expected default MQTT.AutoReconnect true")
	}
	if cfg.MQTT.MaxReconnectInterval != 10*time.Minute {
		t.Errorf("expected default MQTT.MaxReconnectInterval 10m, got %v", cfg.MQTT.MaxReconnectInterval)
	}
	if !cfg.MQTT.Discovery {
		t.Errorf("expected MQTT.Discovery to be true")
	}
	if cfg.SMS.Encoding != "auto" {
		t.Errorf("expected SMS.Encoding 'auto', got %q", cfg.SMS.Encoding)
	}
	if cfg.SMS.LongMessage != "split" {
		t.Errorf("expected SMS.LongMessage 'split', got %q", cfg.SMS.LongMessage)
	}
	if !cfg.SMS.DeliveryReport.Enabled {
		t.Errorf("expected SMS.DeliveryReport.Enabled to be true")
	}
	if cfg.SMS.DeliveryReport.Timeout != 5*time.Minute {
		t.Errorf("expected DeliveryReport.Timeout 5m, got %v", cfg.SMS.DeliveryReport.Timeout)
	}
	if cfg.Security.IncomingFilter != "all" {
		t.Errorf("expected Security.IncomingFilter 'all', got %q", cfg.Security.IncomingFilter)
	}
	if cfg.Security.AllowRawAT {
		t.Errorf("expected Security.AllowRawAT to be false by default")
	}
	if !cfg.Security.RateLimit.Enabled {
		t.Errorf("expected RateLimit.Enabled to be true")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/non/existent/path/gsm2mqtt.yaml")
	if err != nil {
		t.Fatalf("expected non-existent file to load defaults without error, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.MQTT.Broker != "localhost" {
		t.Errorf("expected default broker 'localhost', got %q", cfg.MQTT.Broker)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
log_level: debug
mqtt:
  broker: "192.168.1.50"
  port: 8883
  client_id: "test_client"
  topic_prefix: "my_gsm"
  discovery: false
sms:
  encoding: "ucs2"
  long_message: "truncate"
security:
  incoming_filter: "whitelist"
  whitelist:
    - "+79991112233"
modems:
  - id: "m1"
    port: "/dev/ttyUSB0"
    baud_rate: 115200
    type: "simcom"
`
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("failed to load valid config: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %q", cfg.LogLevel)
	}
	if cfg.MQTT.Broker != "192.168.1.50" {
		t.Errorf("expected broker '192.168.1.50', got %q", cfg.MQTT.Broker)
	}
	if cfg.MQTT.Port != 8883 {
		t.Errorf("expected port 8883, got %d", cfg.MQTT.Port)
	}
	if cfg.SMS.Encoding != "ucs2" {
		t.Errorf("expected encoding 'ucs2', got %q", cfg.SMS.Encoding)
	}
	if len(cfg.Modems) != 1 || cfg.Modems[0].ID != "m1" {
		t.Errorf("unexpected modems list: %+v", cfg.Modems)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("GSM2MQTT_LOG_LEVEL", "error")
	t.Setenv("GSM2MQTT_MQTT_BROKER", "mqtt.local")
	t.Setenv("GSM2MQTT_MQTT_PORT", "1884")
	t.Setenv("GSM2MQTT_MQTT_CLIENT_ID", "env_client")

	cfg, err := Load("/non/existent/path/gsm2mqtt.yaml")
	if err != nil {
		t.Fatalf("failed to load config with env overrides: %v", err)
	}

	if cfg.LogLevel != "error" {
		t.Errorf("expected LogLevel 'error', got %q", cfg.LogLevel)
	}
	if cfg.MQTT.Broker != "mqtt.local" {
		t.Errorf("expected broker 'mqtt.local', got %q", cfg.MQTT.Broker)
	}
	if cfg.MQTT.Port != 1884 {
		t.Errorf("expected port 1884, got %d", cfg.MQTT.Port)
	}
	if cfg.MQTT.ClientID != "env_client" {
		t.Errorf("expected client_id 'env_client', got %q", cfg.MQTT.ClientID)
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErrMsg  string
	}{
		{
			name: "missing broker",
			yamlContent: `
mqtt:
  broker: ""
`,
			wantErrMsg: "mqtt.broker is required",
		},
		{
			name: "invalid port",
			yamlContent: `
mqtt:
  port: 99999
`,
			wantErrMsg: "mqtt.port must be between",
		},
		{
			name: "invalid encoding",
			yamlContent: `
sms:
  encoding: "unknown_enc"
`,
			wantErrMsg: "sms.encoding must be one of",
		},
		{
			name: "invalid filter",
			yamlContent: `
security:
  incoming_filter: "invalid_filter"
`,
			wantErrMsg: "security.incoming_filter must be one of",
		},
		{
			name: "modem missing id",
			yamlContent: `
modems:
  - port: "/dev/ttyS0"
`,
			wantErrMsg: "modems[0].id is required",
		},
		{
			name: "invalid_pool_strategy",
			yamlContent: `
pool:
  enabled: true
  strategy: "invalid_strat"
`,
			wantErrMsg: "pool.strategy must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgFile := filepath.Join(tmpDir, "invalid.yaml")
			if err := os.WriteFile(cfgFile, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			_, err := Load(cfgFile)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
			}
			if !contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoad_PoolDefaultsAndOverrides(t *testing.T) {
	cfg, err := Load("/non/existent")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Pool.Strategy != "round-robin" {
		t.Errorf("expected default pool strategy 'round-robin', got %s", cfg.Pool.Strategy)
	}

	t.Setenv("GSM2MQTT_POOL_ENABLED", "true")
	t.Setenv("GSM2MQTT_POOL_STRATEGY", "failover")
	t.Setenv("GSM2MQTT_POOL_DEFAULT_MODEM", "modem1")

	cfg2, err := Load("/non/existent")
	if err != nil {
		t.Fatalf("load with env failed: %v", err)
	}
	if !cfg2.Pool.Enabled {
		t.Errorf("expected pool enabled true")
	}
	if cfg2.Pool.Strategy != "failover" {
		t.Errorf("expected pool strategy failover, got %s", cfg2.Pool.Strategy)
	}
	if cfg2.Pool.DefaultModem != "modem1" {
		t.Errorf("expected default modem modem1, got %s", cfg2.Pool.DefaultModem)
	}
}

func TestLoad_ModemPortOverride(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	yamlContent := `
modems:
  - id: "m1"
    port: "/dev/ttyUSB0"
`
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Setenv("MODEM_DEVICE", "/dev/ttyACM0")
	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(cfg.Modems) != 1 || cfg.Modems[0].Port != "/dev/ttyACM0" {
		t.Errorf("expected port /dev/ttyACM0, got %v", cfg.Modems[0].Port)
	}
}

func TestLoad_OpenWrtConfig(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "deployments", "openwrt", "files", "gsm2mqtt.yaml")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load OpenWrt configuration file %s: %v", cfgPath, err)
	}
	if len(cfg.Modems) == 0 {
		t.Fatalf("expected at least one modem configured in OpenWrt config")
	}
	if len(cfg.Security.AllowedATCommands) == 0 {
		t.Errorf("expected allowed_at_commands to be configured in OpenWrt config")
	}
	if cfg.Security.RecipientsFile == "" {
		t.Errorf("expected recipients_file to be specified in OpenWrt config")
	}
}

func TestLoad_ExampleConfig(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "configs", "gsm2mqtt.example.yaml")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load example configuration file %s: %v", cfgPath, err)
	}
	if len(cfg.Modems) == 0 {
		t.Fatalf("expected at least one modem configured in example config")
	}
	if len(cfg.Security.AllowedATCommands) == 0 {
		t.Errorf("expected allowed_at_commands to be configured in example config")
	}
	if cfg.Security.RecipientsFile == "" {
		t.Errorf("expected recipients_file to be specified in example config")
	}
	if cfg.MQTT.QoS != 1 {
		t.Errorf("expected example config MQTT.QoS 1, got %d", cfg.MQTT.QoS)
	}
}

func TestLoad_QoSValidation(t *testing.T) {
	tmpDir := t.TempDir()

	validYAML := `
mqtt:
  broker: "localhost"
  port: 1883
  topic_prefix: "test"
  qos: 2
`
	validPath := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(validPath)
	if err != nil {
		t.Fatalf("expected valid config to load, got %v", err)
	}
	if cfg.MQTT.QoS != 2 {
		t.Errorf("expected QoS 2, got %d", cfg.MQTT.QoS)
	}

	invalidYAML := `
mqtt:
  broker: "localhost"
  port: 1883
  topic_prefix: "test"
  qos: 3
`
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(invalidPath); err == nil {
		t.Fatal("expected error for qos = 3, got nil")
	}
}

func TestLoad_MQTTParameters(t *testing.T) {
	tmpDir := t.TempDir()
	content := `
mqtt:
  broker: "broker.lan"
  port: 1883
  topic_prefix: "gsm"
  qos: 0
  clean_session: false
  keep_alive: 45s
  connect_timeout: 5s
  auto_reconnect: false
  max_reconnect_interval: 2m
`
	p := filepath.Join(tmpDir, "params.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if cfg.MQTT.QoS != 0 {
		t.Errorf("expected QoS 0, got %d", cfg.MQTT.QoS)
	}
	if cfg.MQTT.CleanSession != false {
		t.Errorf("expected CleanSession false, got %v", cfg.MQTT.CleanSession)
	}
	if cfg.MQTT.KeepAlive != 45*time.Second {
		t.Errorf("expected KeepAlive 45s, got %v", cfg.MQTT.KeepAlive)
	}
	if cfg.MQTT.ConnectTimeout != 5*time.Second {
		t.Errorf("expected ConnectTimeout 5s, got %v", cfg.MQTT.ConnectTimeout)
	}
	if cfg.MQTT.AutoReconnect != false {
		t.Errorf("expected AutoReconnect false, got %v", cfg.MQTT.AutoReconnect)
	}
	if cfg.MQTT.MaxReconnectInterval != 2*time.Minute {
		t.Errorf("expected MaxReconnectInterval 2m, got %v", cfg.MQTT.MaxReconnectInterval)
	}
}



