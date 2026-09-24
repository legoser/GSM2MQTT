package config

import (
	"strconv"
	"strings"
	"time"
)

// applyHierarchicalOverrides applies configuration overrides with priority: OS Env > .env > YAML.
func applyHierarchicalOverrides(cfg *Config, dotEnv map[string]string) {
	applyServiceOverrides(cfg, dotEnv)
	applyMQTTOverrides(cfg, dotEnv)
	applyModemOverrides(cfg, dotEnv)
	applySecurityOverrides(cfg, dotEnv)
	applySMSOverrides(cfg, dotEnv)
	applyStatusOverrides(cfg, dotEnv)
	applyTariffOverrides(cfg, dotEnv)
}

func applyServiceOverrides(cfg *Config, dotEnv map[string]string) {
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_LOG_LEVEL", "LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_API_ENABLED", "API_ENABLED"); v != "" {
		cfg.API.Enabled = parseBool(v, cfg.API.Enabled)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_API_HOST", "API_HOST"); v != "" {
		cfg.API.Host = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_API_PORT", "API_PORT"); v != "" {
		cfg.API.Port = atoi(v, cfg.API.Port)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_API_TOKEN", "API_TOKEN"); v != "" {
		cfg.API.Token = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_POOL_ENABLED", "POOL_ENABLED"); v != "" {
		cfg.Pool.Enabled = parseBool(v, cfg.Pool.Enabled)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_POOL_STRATEGY", "POOL_STRATEGY"); v != "" {
		cfg.Pool.Strategy = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_POOL_DEFAULT_MODEM", "POOL_DEFAULT_MODEM"); v != "" {
		cfg.Pool.DefaultModem = v
	}
}

func applyMQTTOverrides(cfg *Config, dotEnv map[string]string) {
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_BROKER", "MQTT_BROKER"); v != "" {
		cfg.MQTT.Broker = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_PORT", "MQTT_PORT"); v != "" {
		cfg.MQTT.Port = atoi(v, cfg.MQTT.Port)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_USERNAME", "MQTT_USERNAME"); v != "" {
		cfg.MQTT.Username = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_PASSWORD", "MQTT_PASSWORD"); v != "" {
		cfg.MQTT.Password = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_CLIENT_ID", "MQTT_CLIENT_ID"); v != "" {
		cfg.MQTT.ClientID = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_TOPIC_PREFIX", "MQTT_TOPIC_PREFIX"); v != "" {
		cfg.MQTT.TopicPrefix = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_DISCOVERY", "MQTT_DISCOVERY"); v != "" {
		cfg.MQTT.Discovery = parseBool(v, cfg.MQTT.Discovery)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_DISCOVERY_PREFIX", "MQTT_DISCOVERY_PREFIX"); v != "" {
		cfg.MQTT.DiscoveryPrefix = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_TLS_ENABLED", "MQTT_TLS_ENABLED"); v != "" {
		cfg.MQTT.TLS.Enabled = parseBool(v, cfg.MQTT.TLS.Enabled)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_TLS_CA_CERT", "MQTT_TLS_CA_CERT"); v != "" {
		cfg.MQTT.TLS.CACert = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_TLS_CLIENT_CERT", "MQTT_TLS_CLIENT_CERT"); v != "" {
		cfg.MQTT.TLS.ClientCert = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_TLS_CLIENT_KEY", "MQTT_TLS_CLIENT_KEY"); v != "" {
		cfg.MQTT.TLS.ClientKey = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_MQTT_TLS_INSECURE_SKIP_VERIFY", "MQTT_TLS_INSECURE_SKIP_VERIFY"); v != "" {
		cfg.MQTT.TLS.InsecureSkipVerify = parseBool(v, cfg.MQTT.TLS.InsecureSkipVerify)
	}
}

func applyModemOverrides(cfg *Config, dotEnv map[string]string) {
	port := getHierarchicalValue(dotEnv, "GSM2MQTT_MODEM_PORT", "MODEM_DEVICE", "MODEM_PORT")
	id := getHierarchicalValue(dotEnv, "GSM2MQTT_MODEM_ID", "MODEM_ID")
	name := getHierarchicalValue(dotEnv, "GSM2MQTT_MODEM_NAME", "MODEM_NAME")
	mType := getHierarchicalValue(dotEnv, "GSM2MQTT_MODEM_TYPE", "MODEM_TYPE")
	baud := getHierarchicalValue(dotEnv, "GSM2MQTT_MODEM_BAUD_RATE", "MODEM_BAUD_RATE")
	pin := getHierarchicalValue(dotEnv, "GSM2MQTT_MODEM_PIN", "MODEM_PIN")

	if port == "" && id == "" && name == "" && mType == "" && baud == "" && pin == "" {
		return
	}

	m := ensureDefaultModem(cfg)
	if port != "" {
		m.Port = port
	}
	if id != "" {
		m.ID = id
	}
	if name != "" {
		m.Name = name
	}
	if mType != "" {
		m.Type = mType
	}
	if baud != "" {
		m.BaudRate = atoi(baud, m.BaudRate)
	}
	if pin != "" {
		m.PIN = pin
	}
}

func ensureDefaultModem(cfg *Config) *ModemConfig {
	if len(cfg.Modems) == 0 {
		cfg.Modems = []ModemConfig{
			{
				ID:          "modem1",
				Name:        "Default Modem",
				Port:        "",
				BaudRate:    9600,
				Type:        "auto",
				DataBits:    8,
				StopBits:    1,
				Parity:      "none",
				FlowControl: "none",
			},
		}
	}
	return &cfg.Modems[0]
}

func applySecurityOverrides(cfg *Config, dotEnv map[string]string) {
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_INCOMING_FILTER", "SECURITY_INCOMING_FILTER"); v != "" {
		cfg.Security.IncomingFilter = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_ALLOW_RAW_AT", "SECURITY_ALLOW_RAW_AT"); v != "" {
		cfg.Security.AllowRawAT = parseBool(v, cfg.Security.AllowRawAT)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_RATE_LIMIT_ENABLED", "GSM2MQTT_RATE_LIMIT_ENABLED", "RATE_LIMIT_ENABLED"); v != "" {
		cfg.Security.RateLimit.Enabled = parseBool(v, cfg.Security.RateLimit.Enabled)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_RATE_LIMIT_MAX_SMS_PER_MINUTE", "GSM2MQTT_MAX_SMS_PER_MINUTE", "MAX_SMS_PER_MINUTE"); v != "" {
		cfg.Security.RateLimit.MaxSMSPerMinute = atoi(v, cfg.Security.RateLimit.MaxSMSPerMinute)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_RATE_LIMIT_MAX_SMS_PER_HOUR", "GSM2MQTT_MAX_SMS_PER_HOUR", "MAX_SMS_PER_HOUR"); v != "" {
		cfg.Security.RateLimit.MaxSMSPerHour = atoi(v, cfg.Security.RateLimit.MaxSMSPerHour)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_RATE_LIMIT_MAX_SMS_PER_DAY", "GSM2MQTT_MAX_SMS_PER_DAY", "MAX_SMS_PER_DAY"); v != "" {
		cfg.Security.RateLimit.MaxSMSPerDay = atoi(v, cfg.Security.RateLimit.MaxSMSPerDay)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_RATE_LIMIT_MAX_SMS_PER_NUMBER_PER_HOUR", "GSM2MQTT_MAX_SMS_PER_NUMBER_PER_HOUR"); v != "" {
		cfg.Security.RateLimit.MaxSMSPerNumberPerHour = atoi(v, cfg.Security.RateLimit.MaxSMSPerNumberPerHour)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SECURITY_RATE_LIMIT_COOLDOWN_MINUTES", "GSM2MQTT_RATE_LIMIT_COOLDOWN_MINUTES"); v != "" {
		cfg.Security.RateLimit.CooldownMinutes = atoi(v, cfg.Security.RateLimit.CooldownMinutes)
	}
}

func applySMSOverrides(cfg *Config, dotEnv map[string]string) {
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_ENCODING", "SMS_ENCODING"); v != "" {
		cfg.SMS.Encoding = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_LONG_MESSAGE", "SMS_LONG_MESSAGE"); v != "" {
		cfg.SMS.LongMessage = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_MAX_SEGMENTS", "SMS_MAX_SEGMENTS"); v != "" {
		cfg.SMS.MaxSegments = atoi(v, cfg.SMS.MaxSegments)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_REPORT_ENCODING", "SMS_REPORT_ENCODING"); v != "" {
		cfg.SMS.ReportEncoding = parseBool(v, cfg.SMS.ReportEncoding)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_DELIVERY_REPORT_ENABLED", "SMS_DELIVERY_REPORT_ENABLED"); v != "" {
		cfg.SMS.DeliveryReport.Enabled = parseBool(v, cfg.SMS.DeliveryReport.Enabled)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_DELIVERY_REPORT_TIMEOUT", "SMS_DELIVERY_REPORT_TIMEOUT"); v != "" {
		cfg.SMS.DeliveryReport.Timeout = parseDuration(v, cfg.SMS.DeliveryReport.Timeout)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_SMS_DELIVERY_REPORT_PUBLISH_PENDING", "SMS_DELIVERY_REPORT_PUBLISH_PENDING"); v != "" {
		cfg.SMS.DeliveryReport.PublishPending = parseBool(v, cfg.SMS.DeliveryReport.PublishPending)
	}
}

func applyStatusOverrides(cfg *Config, dotEnv map[string]string) {
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_STATUS_INTERVAL", "STATUS_INTERVAL"); v != "" {
		cfg.Status.Interval = parseDuration(v, cfg.Status.Interval)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_STATUS_SIGNAL_INTERVAL", "STATUS_SIGNAL_INTERVAL"); v != "" {
		cfg.Status.SignalInterval = parseDuration(v, cfg.Status.SignalInterval)
	}
}

func applyTariffOverrides(cfg *Config, dotEnv map[string]string) {
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_ENABLED", "TARIFF_ENABLED"); v != "" {
		cfg.Tariff.Enabled = parseBool(v, cfg.Tariff.Enabled)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_OPERATOR_PRESET", "TARIFF_OPERATOR_PRESET"); v != "" {
		cfg.Tariff.OperatorPreset = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_BALANCE_USSD", "TARIFF_BALANCE_USSD"); v != "" {
		cfg.Tariff.BalanceUSSD = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_BALANCE_REGEX", "TARIFF_BALANCE_REGEX"); v != "" {
		cfg.Tariff.BalanceRegex = v
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_AUTO_CHECK_ON_ERROR", "TARIFF_AUTO_CHECK_ON_ERROR"); v != "" {
		cfg.Tariff.AutoCheckOnError = parseBool(v, cfg.Tariff.AutoCheckOnError)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_CHECK_INTERVAL", "TARIFF_CHECK_INTERVAL"); v != "" {
		cfg.Tariff.CheckInterval = parseDuration(v, cfg.Tariff.CheckInterval)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_MIN_BALANCE_ALERT", "TARIFF_MIN_BALANCE_ALERT"); v != "" {
		cfg.Tariff.MinBalanceAlert = parseFloat(v, cfg.Tariff.MinBalanceAlert)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_SMS_LIMIT", "TARIFF_SMS_LIMIT"); v != "" {
		cfg.Tariff.SMSLimit = atoi(v, cfg.Tariff.SMSLimit)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_CALL_MINUTES_LIMIT", "TARIFF_CALL_MINUTES_LIMIT"); v != "" {
		cfg.Tariff.CallMinutesLimit = parseFloat(v, cfg.Tariff.CallMinutesLimit)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_DATA_TRAFFIC_LIMIT_MB", "TARIFF_DATA_TRAFFIC_LIMIT_MB"); v != "" {
		cfg.Tariff.DataTrafficLimitMB = parseInt64(v, cfg.Tariff.DataTrafficLimitMB)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_RESET_DAY_OF_MONTH", "TARIFF_RESET_DAY_OF_MONTH"); v != "" {
		cfg.Tariff.ResetDayOfMonth = atoi(v, cfg.Tariff.ResetDayOfMonth)
	}
	if v := getHierarchicalValue(dotEnv, "GSM2MQTT_TARIFF_STORAGE_DIR", "TARIFF_STORAGE_DIR"); v != "" {
		cfg.Tariff.StorageDir = v
	}
}

func parseBool(s string, defaultVal bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultVal
	}
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	if sec, err := strconv.Atoi(s); err == nil {
		return time.Duration(sec) * time.Second
	}
	return defaultVal
}

func parseFloat(s string, defaultVal float64) float64 {
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return f
	}
	return defaultVal
}

func parseInt64(s string, defaultVal int64) int64 {
	if i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return i
	}
	return defaultVal
}

func atoi(s string, defaultVal int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return v
	}
	return defaultVal
}
