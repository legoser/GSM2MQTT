package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnv_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `
# Comment line
MODEM_DEVICE=/dev/ttyUSB0
API_PORT=8080
MQTT_BROKER="192.168.1.10"
MQTT_PORT='1883'
EMPTY_VAL=
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("write .env failed: %v", err)
	}

	envMap := parseEnvFile(envPath)
	if envMap["MODEM_DEVICE"] != "/dev/ttyUSB0" {
		t.Errorf("expected /dev/ttyUSB0, got %q", envMap["MODEM_DEVICE"])
	}
	if envMap["API_PORT"] != "8080" {
		t.Errorf("expected 8080, got %q", envMap["API_PORT"])
	}
	if envMap["MQTT_BROKER"] != "192.168.1.10" {
		t.Errorf("expected 192.168.1.10, got %q", envMap["MQTT_BROKER"])
	}
	if envMap["MQTT_PORT"] != "1883" {
		t.Errorf("expected 1883, got %q", envMap["MQTT_PORT"])
	}
}

func TestHierarchy_EnvOverridesDotEnvAndYaml(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gsm2mqtt.yaml")
	envPath := filepath.Join(tmpDir, ".env")

	yamlContent := `
mqtt:
  broker: "yaml-broker"
  port: 1883
api:
  port: 8080
modems:
  - id: "m1"
    port: "/dev/yaml-port"
`
	dotEnvContent := `
MQTT_BROKER=dotenv-broker
API_PORT=9090
MODEM_DEVICE=/dev/dotenv-port
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write yaml failed: %v", err)
	}
	if err := os.WriteFile(envPath, []byte(dotEnvContent), 0644); err != nil {
		t.Fatalf("write .env failed: %v", err)
	}

	// 1. YAML + .env (no OS env yet)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.MQTT.Broker != "dotenv-broker" {
		t.Errorf("expected dotenv-broker, got %q", cfg.MQTT.Broker)
	}
	if cfg.API.Port != 9090 {
		t.Errorf("expected 9090, got %d", cfg.API.Port)
	}
	if len(cfg.Modems) != 1 || cfg.Modems[0].Port != "/dev/dotenv-port" {
		t.Errorf("expected /dev/dotenv-port, got %v", cfg.Modems[0].Port)
	}

	// 2. OS Env overrides .env and YAML
	t.Setenv("MQTT_BROKER", "os-env-broker")
	t.Setenv("MODEM_DEVICE", "/dev/os-env-port")

	cfg2, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg2.MQTT.Broker != "os-env-broker" {
		t.Errorf("expected os-env-broker to override .env, got %q", cfg2.MQTT.Broker)
	}
	if cfg2.Modems[0].Port != "/dev/os-env-port" {
		t.Errorf("expected /dev/os-env-port to override .env, got %q", cfg2.Modems[0].Port)
	}
	// API_PORT wasn't set in OS Env, so it should still come from .env
	if cfg2.API.Port != 9090 {
		t.Errorf("expected 9090 from .env, got %d", cfg2.API.Port)
	}
}
