package mqtt

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
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

// PahoClient wraps the official Eclipse Paho MQTT client library.
type PahoClient struct {
	client paho.Client
	cfg    ClientConfig
}

// NewPahoClient constructs a Paho-backed MQTT client.
func NewPahoClient(cfg ClientConfig) (*PahoClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	if cfg.ClientID != "" {
		opts.SetClientID(cfg.ClientID)
	} else {
		opts.SetClientID("gsm2mqtt-gateway")
	}

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	if cfg.LWTTopic != "" {
		lwtQoS := cfg.LWTQoS
		if lwtQoS == 0 && cfg.QoS != 0 {
			lwtQoS = cfg.QoS
		}
		opts.SetWill(cfg.LWTTopic, cfg.LWTPayload, lwtQoS, cfg.LWTRetained)
	}

	if cfg.TLSEnabled {
		opts.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: cfg.InsecureTLS,
		})
	}

	opts.SetCleanSession(cfg.CleanSession)
	opts.SetAutoReconnect(cfg.AutoReconnect)

	if cfg.KeepAlive > 0 {
		opts.SetKeepAlive(cfg.KeepAlive)
	} else {
		opts.SetKeepAlive(60 * time.Second)
	}

	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts.SetConnectTimeout(timeout)

	if cfg.MaxReconnectInterval > 0 {
		opts.SetMaxReconnectInterval(cfg.MaxReconnectInterval)
	}

	opts.SetOnConnectHandler(func(_ paho.Client) {
		slog.Info("connected to MQTT broker", slog.String("broker", cfg.Broker), slog.String("client_id", cfg.ClientID))
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		slog.Warn("connection lost to MQTT broker", slog.String("broker", cfg.Broker), slog.Any("error", err))
	})
	opts.SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
		slog.Info("reconnecting to MQTT broker...", slog.String("broker", cfg.Broker))
	})

	client := paho.NewClient(opts)
	return &PahoClient{client: client, cfg: cfg}, nil
}

// Connect connects to the MQTT broker.
func (c *PahoClient) Connect() error {
	slog.Info("connecting to MQTT broker", slog.String("broker", c.cfg.Broker))
	token := c.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT operation timed out")
	}
	if token.Error() != nil {
		slog.Error("failed to connect to MQTT broker", slog.String("broker", c.cfg.Broker), slog.Any("error", token.Error()))
		return token.Error()
	}
	return nil
}

// Disconnect gracefully disconnects from the MQTT broker.
func (c *PahoClient) Disconnect(quiesce uint) {
	slog.Info("disconnecting from MQTT broker", slog.String("broker", c.cfg.Broker))
	c.client.Disconnect(quiesce)
}

// Publish publishes a message to a topic.
func (c *PahoClient) Publish(topic string, qos byte, retained bool, payload []byte) error {
	slog.Debug("publishing MQTT message",
		slog.String("topic", topic),
		slog.Int("bytes", len(payload)),
		slog.Int("qos", int(qos)),
		slog.Bool("retained", retained),
	)
	if !c.IsConnected() {
		slog.Error("cannot publish: MQTT client not connected", slog.String("topic", topic))
		return ErrNotConnected
	}
	token := c.client.Publish(topic, qos, retained, payload)
	timeout := 10 * time.Second
	if c.cfg.ConnectTimeout > 0 {
		timeout = c.cfg.ConnectTimeout
	}
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("MQTT operation timed out")
	}
	if token.Error() != nil {
		slog.Error("MQTT publish failed", slog.String("topic", topic), slog.Any("error", token.Error()))
		return token.Error()
	}
	return nil
}

// Subscribe subscribes to a topic pattern.
func (c *PahoClient) Subscribe(topic string, qos byte, handler MessageHandler) error {
	slog.Debug("subscribing to MQTT topic", slog.String("topic", topic), slog.Int("qos", int(qos)))
	token := c.client.Subscribe(topic, qos, func(_ paho.Client, m paho.Message) {
		slog.Debug("received MQTT message", slog.String("topic", m.Topic()), slog.Int("bytes", len(m.Payload())))
		handler(m.Topic(), m.Payload())
	})
	timeout := 10 * time.Second
	if c.cfg.ConnectTimeout > 0 {
		timeout = c.cfg.ConnectTimeout
	}
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("MQTT operation timed out")
	}
	if token.Error() != nil {
		slog.Error("MQTT subscribe failed", slog.String("topic", topic), slog.Any("error", token.Error()))
		return token.Error()
	}
	return nil
}

// IsConnected returns whether the client is connected.
func (c *PahoClient) IsConnected() bool {
	return c.client.IsConnected()
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
}

// NewMockClient creates a new mock MQTT client.
func NewMockClient() *MockClient {
	return &MockClient{
		subscribers: make(map[string][]MessageHandler),
	}
}

// Connect simulates client connection.
func (m *MockClient) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
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

var _ MQTTClient = (*PahoClient)(nil)
var _ MQTTClient = (*MockClient)(nil)
