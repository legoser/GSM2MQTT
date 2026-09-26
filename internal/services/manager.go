package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/mqtt"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

// GatewayManager coordinates multiple ModemRunner instances and provides an aggregate facade.
type GatewayManager struct {
	mu            sync.RWMutex
	runners       map[string]*ModemRunner
	order         []string
	mqttClient    mqtt.MQTTClient
	mqttCfg       config.MQTTConfig
	recipientsMgr *security.RecipientsManager
}

// NewGatewayManager creates a new GatewayManager.
func NewGatewayManager() *GatewayManager {
	return &GatewayManager{
		runners: make(map[string]*ModemRunner),
		mqttCfg: config.MQTTConfig{
			QoS:          1,
			CleanSession: true,
		},
	}
}

// Register adds a ModemRunner to the manager.
func (m *GatewayManager) Register(runner *ModemRunner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runners[runner.mCfg.ID] = runner
	m.order = append(m.order, runner.mCfg.ID)
}

// GetModems returns the summaries of all registered modems.
func (m *GatewayManager) GetModems() []ModemSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]ModemSummary, 0, len(m.runners))
	for _, id := range m.order {
		if r, ok := m.runners[id]; ok {
			summaries = append(summaries, r.Summary())
		}
	}
	return summaries
}

// SendSMS dispatches an SMS through the requested (or first available) modem.
func (m *GatewayManager) SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error) {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return nil, err
	}
	return runner.SendSMS(ctx, to, text)
}

// ReceivedSMS represents an incoming SMS message cached for inspection.
type ReceivedSMS struct {
	ID        string `json:"id"`
	ModemID   string `json:"modem_id"`
	Sender    string `json:"sender"`
	From      string `json:"from,omitempty"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

// DialCall initiates an outgoing voice call on the specified (or first) modem.
func (m *GatewayManager) DialCall(ctx context.Context, modemID, number string) error {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return err
	}
	return runner.Dial(ctx, number)
}

// HangupCall terminates active voice calls on the specified (or first) modem.
func (m *GatewayManager) HangupCall(ctx context.Context, modemID string) error {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return err
	}
	return runner.Hangup(ctx)
}

// GetCallStatus returns the real-time call status for the requested modem.
func (m *GatewayManager) GetCallStatus(modemID string) CallStatus {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return CallStatus{State: CallStateIdle, Message: "Modem not found"}
	}
	return runner.GetCallStatus()
}

// GetReceivedSMS collects all recent received SMS across all modems.
func (m *GatewayManager) GetReceivedSMS() []ReceivedSMS {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]ReceivedSMS, 0)
	for _, id := range m.order {
		if r, ok := m.runners[id]; ok {
			all = append(all, r.GetReceivedSMS()...)
		}
	}
	return all
}

// ClearReceivedSMS wipes the received SMS inbox for a specific modem or all modems if modemID is empty or "all".
func (m *GatewayManager) ClearReceivedSMS(modemID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modemID != "" && modemID != "all" {
		r, ok := m.runners[modemID]
		if !ok {
			return fmt.Errorf("modem %s not found", modemID)
		}
		r.ClearReceivedSMS()
		return nil
	}

	for _, r := range m.runners {
		r.ClearReceivedSMS()
	}
	return nil
}

// GetCallHistory collects recorded voice calls across all modems or for a specific modem.
func (m *GatewayManager) GetCallHistory(modemID string) []CallRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modemID != "" && modemID != "all" {
		if r, ok := m.runners[modemID]; ok {
			return r.GetCallHistory()
		}
		return []CallRecord{}
	}

	all := make([]CallRecord, 0)
	for _, id := range m.order {
		if r, ok := m.runners[id]; ok {
			all = append(all, r.GetCallHistory()...)
		}
	}
	return all
}

// ClearCallHistory wipes recorded voice calls for a specific modem or all modems if modemID is empty or "all".
func (m *GatewayManager) ClearCallHistory(modemID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modemID != "" && modemID != "all" {
		r, ok := m.runners[modemID]
		if !ok {
			return fmt.Errorf("modem %s not found", modemID)
		}
		r.ClearCallHistory()
		return nil
	}

	for _, r := range m.runners {
		r.ClearCallHistory()
	}
	return nil
}

// SendUSSD dispatches a USSD query through the requested (or first available) modem.
func (m *GatewayManager) SendUSSD(ctx context.Context, modemID, code string) (string, error) {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return "", err
	}
	return runner.SendUSSD(ctx, code)
}

// SendRawAT sends an arbitrary AT command through the requested (or first available) modem.
func (m *GatewayManager) SendRawAT(ctx context.Context, modemID, cmd string) (string, error) {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return "", err
	}
	return runner.SendRawAT(ctx, cmd)
}

// UpdateTariffConfig updates tariff settings on the requested (or first available) modem.
func (m *GatewayManager) UpdateTariffConfig(modemID string, cfg tariff.Config) error {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return err
	}
	return runner.UpdateTariffConfig(cfg)
}

// SetTariffUsage manually updates tariff usage counters on the requested (or first available) modem.
func (m *GatewayManager) SetTariffUsage(modemID string, update tariff.UsageUpdate) error {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return err
	}
	return runner.SetTariffUsage(update)
}

// ResetTariffQuotas resets monthly quota counters on the requested (or first available) modem.
func (m *GatewayManager) ResetTariffQuotas(modemID string) error {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return err
	}
	return runner.ResetTariffQuotas()
}

// GetTariffStatus returns current tariff usage statistics on the requested (or first available) modem.
func (m *GatewayManager) GetTariffStatus(modemID string) (*tariff.UsageStatus, error) {
	runner, err := m.findRunner(modemID)
	if err != nil {
		return nil, err
	}
	return runner.GetTariffStatus()
}

func (m *GatewayManager) findRunner(modemID string) (*ModemRunner, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modemID != "" {
		if r, ok := m.runners[modemID]; ok {
			return r, nil
		}
		return nil, fmt.Errorf("modem %q not found", modemID)
	}

	if len(m.order) > 0 {
		return m.runners[m.order[0]], nil
	}
	return nil, fmt.Errorf("no modems available")
}

// MQTTStatus contains connection state and non-sensitive configuration for the MQTT broker.
type MQTTStatus struct {
	Connected        bool   `json:"connected"`
	Broker           string `json:"broker"`
	Port             int    `json:"port"`
	ClientID         string `json:"client_id"`
	TopicPrefix      string `json:"topic_prefix"`
	Username         string `json:"username,omitempty"`
	QoS              int    `json:"qos"`
	CleanSession     bool   `json:"clean_session"`
	KeepAliveSeconds int    `json:"keep_alive_seconds,omitempty"`
	Discovery        bool   `json:"discovery"`
	DiscoveryPrefix  string `json:"discovery_prefix,omitempty"`
}

// SetMQTT stores the MQTT client and configuration for reporting and binds recipients.
func (m *GatewayManager) SetMQTT(client mqtt.MQTTClient, cfg *config.MQTTConfig) {
	m.mu.Lock()
	m.mqttClient = client
	if cfg != nil {
		m.mqttCfg = *cfg
	}
	topicPrefix := m.mqttCfg.TopicPrefix
	m.mu.Unlock()

	if client != nil && client.IsConnected() && topicPrefix != "" {
		m.subscribeRecipientsMQTT(client, topicPrefix)
		m.publishRecipientsState(m.GetRecipients())
	}
}

// GetMQTTStatus returns the current connection state and broker configuration.
func (m *GatewayManager) GetMQTTStatus() MQTTStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connected := false
	if m.mqttClient != nil {
		connected = m.mqttClient.IsConnected()
	}
	return MQTTStatus{
		Connected:        connected,
		Broker:           m.mqttCfg.Broker,
		Port:             m.mqttCfg.Port,
		ClientID:         m.mqttCfg.ClientID,
		TopicPrefix:      m.mqttCfg.TopicPrefix,
		Username:         m.mqttCfg.Username,
		QoS:              m.mqttCfg.QoS,
		CleanSession:     m.mqttCfg.CleanSession,
		KeepAliveSeconds: int(m.mqttCfg.KeepAlive.Seconds()),
		Discovery:        m.mqttCfg.Discovery,
		DiscoveryPrefix:  m.mqttCfg.DiscoveryPrefix,
	}
}

func (m *GatewayManager) qos() byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return byte(m.mqttCfg.QoS)
}

// PublishDiscovery republishes discovery for all registered modem runners.
func (m *GatewayManager) PublishDiscovery() {
	m.mu.RLock()
	runners := make([]*ModemRunner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.RUnlock()

	for _, r := range runners {
		r.PublishDiscovery()
	}
}

// PublishRecipientsState republishes current recipient list to MQTT.
func (m *GatewayManager) PublishRecipientsState() {
	m.publishRecipientsState(m.GetRecipients())
}
