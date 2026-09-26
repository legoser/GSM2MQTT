package mqtt

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MessageHandler handles incoming MQTT topic payloads.
type MessageHandler func(topic string, payload []byte)

// ClientConfig holds settings for establishing MQTT broker sessions.
type ClientConfig struct {
	Broker               string
	Port                 int
	Username             string
	Password             string
	ClientID             string
	QoS                  byte
	CleanSession         bool
	KeepAlive            time.Duration
	ConnectTimeout       time.Duration
	AutoReconnect        bool
	MaxReconnectInterval time.Duration
	LWTTopic             string
	LWTPayload           string
	LWTQoS               byte
	LWTRetained          bool
	TLSEnabled           bool
	InsecureTLS          bool
	OnConnect            func(client MQTTClient)
}

// Validate checks essential MQTT client configuration parameters.
func (c *ClientConfig) Validate() error {
	if strings.TrimSpace(c.Broker) == "" {
		return fmt.Errorf("broker address cannot be empty")
	}
	if c.QoS > 2 {
		return fmt.Errorf("invalid QoS %d: must be 0, 1, or 2", c.QoS)
	}
	return nil
}

// MQTTClient abstracts the MQTT transport for testability.
type MQTTClient interface {
	Connect() error
	Disconnect(quiesce uint)
	Publish(topic string, qos byte, retained bool, payload []byte) error
	Subscribe(topic string, qos byte, handler MessageHandler) error
	IsConnected() bool
}

// PublishedMessage records an invocation of Publish for inspection in tests.
type PublishedMessage struct {
	Topic    string
	QoS      byte
	Retained bool
	Payload  []byte
}

// MockClient implements an in-memory MQTTClient for tests.
type MockClient struct {
	mu          sync.RWMutex
	connected   bool
	subscribers map[string][]MessageHandler
	published   []PublishedMessage
	onConnect   func(client MQTTClient)
}

// NewMockClient creates a new mock MQTT client.
func NewMockClient() *MockClient {
	return &MockClient{
		subscribers: make(map[string][]MessageHandler),
	}
}

// SetOnConnect sets the callback invoked when the client connects.
func (m *MockClient) SetOnConnect(fn func(client MQTTClient)) {
	m.mu.Lock()
	m.onConnect = fn
	m.mu.Unlock()
}

// Connect simulates client connection.
func (m *MockClient) Connect() error {
	m.mu.Lock()
	m.connected = true
	fn := m.onConnect
	m.mu.Unlock()
	if fn != nil {
		fn(m)
	}
	return nil
}

// Disconnect simulates client disconnection.
func (m *MockClient) Disconnect(_ uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
}

// Publish routes the payload in-memory to subscribed handlers and records the message.
func (m *MockClient) Publish(topic string, qos byte, retained bool, payload []byte) error {
	m.mu.Lock()
	if !m.connected {
		m.mu.Unlock()
		return ErrNotConnected
	}
	m.published = append(m.published, PublishedMessage{
		Topic:    topic,
		QoS:      qos,
		Retained: retained,
		Payload:  payload,
	})
	handlers := append([]MessageHandler(nil), m.subscribers[topic]...)
	m.mu.Unlock()

	for _, h := range handlers {
		h(topic, payload)
	}
	return nil
}

// Published returns a snapshot of all messages published to this mock client.
func (m *MockClient) Published() []PublishedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]PublishedMessage, len(m.published))
	copy(res, m.published)
	return res
}

// Subscribe registers an in-memory topic handler.
func (m *MockClient) Subscribe(topic string, qos byte, handler MessageHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers[topic] = append(m.subscribers[topic], handler)
	return nil
}

// IsConnected returns the current mock connection state.
func (m *MockClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

var _ MQTTClient = (*MockClient)(nil)
