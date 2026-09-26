//go:build !no_tls

package mqtt

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// PahoClient wraps the official Eclipse Paho MQTT client library.
type PahoClient struct {
	client paho.Client
	cfg    ClientConfig
}

// NewClient constructs a standard MQTT client (backed by Eclipse Paho with TLS support).
func NewClient(cfg ClientConfig) (MQTTClient, error) {
	return NewPahoClient(cfg)
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

var _ MQTTClient = (*PahoClient)(nil)
