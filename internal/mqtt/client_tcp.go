//go:build no_tls

package mqtt

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

// TCPClient is a lightweight, pure-TCP MQTT 3.1.1 client that excludes crypto/tls
// and crypto/rand to operate reliably on resource-constrained embedded targets (e.g. MIPS 24Kc).
type TCPClient struct {
	cfg ClientConfig

	mu          sync.RWMutex
	conn        net.Conn
	connected   atomic.Bool
	closed      atomic.Bool
	subscribers map[string][]MessageHandler

	writeMu sync.Mutex
	ackMu   sync.Mutex
	pubacks map[uint16]chan struct{}
	subacks map[uint16]chan struct{}
	nextID  atomic.Uint32

	doneCh chan struct{}
}

// NewClient constructs an MQTT client. Under no_tls, this produces a pure-TCP client.
func NewClient(cfg ClientConfig) (MQTTClient, error) {
	return NewTCPClient(cfg)
}

// NewPahoClient is provided for API compatibility under no_tls build.
func NewPahoClient(cfg ClientConfig) (MQTTClient, error) {
	return NewTCPClient(cfg)
}

// NewTCPClient constructs a new pure TCP MQTT client.
func NewTCPClient(cfg ClientConfig) (*TCPClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &TCPClient{
		cfg:         cfg,
		subscribers: make(map[string][]MessageHandler),
		pubacks:     make(map[uint16]chan struct{}),
		subacks:     make(map[uint16]chan struct{}),
		doneCh:      make(chan struct{}),
	}, nil
}

func (c *TCPClient) resolveBrokerAddress() string {
	broker := c.cfg.Broker
	broker = strings.TrimPrefix(broker, "tcp://")
	broker = strings.TrimPrefix(broker, "ssl://")
	broker = strings.TrimPrefix(broker, "tls://")
	broker = strings.TrimPrefix(broker, "mqtt://")

	if !strings.Contains(broker, ":") {
		port := c.cfg.Port
		if port <= 0 {
			port = 1883
		}
		return fmt.Sprintf("%s:%d", broker, port)
	}
	return broker
}

func (c *TCPClient) nextMsgID() uint16 {
	for {
		val := uint16(c.nextID.Add(1) & 0xFFFF)
		if val != 0 {
			return val
		}
	}
}

// Connect dials the MQTT broker and executes the MQTT 3.1.1 CONNECT handshake.
func (c *TCPClient) Connect() error {
	addr := c.resolveBrokerAddress()
	timeout := c.cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	slog.Info("connecting to MQTT broker (pure TCP)", slog.String("broker", addr))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("dial MQTT broker %s: %w", addr, err)
	}

	cp := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	cp.ProtocolName = "MQTT"
	cp.ProtocolVersion = 4
	cp.CleanSession = c.cfg.CleanSession

	if c.cfg.ClientID != "" {
		cp.ClientIdentifier = c.cfg.ClientID
	} else {
		cp.ClientIdentifier = "gsm2mqtt-gateway"
	}

	keepAlive := c.cfg.KeepAlive
	if keepAlive <= 0 {
		keepAlive = 60 * time.Second
	}
	cp.Keepalive = uint16(keepAlive.Seconds())

	if c.cfg.Username != "" {
		cp.UsernameFlag = true
		cp.Username = c.cfg.Username
		if c.cfg.Password != "" {
			cp.PasswordFlag = true
			cp.Password = []byte(c.cfg.Password)
		}
	}

	if c.cfg.LWTTopic != "" {
		cp.WillFlag = true
		cp.WillTopic = c.cfg.LWTTopic
		cp.WillMessage = []byte(c.cfg.LWTPayload)
		lwtQoS := c.cfg.LWTQoS
		if lwtQoS == 0 && c.cfg.QoS != 0 {
			lwtQoS = c.cfg.QoS
		}
		cp.WillQos = lwtQoS
		cp.WillRetain = c.cfg.LWTRetained
	}

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if err := cp.Write(conn); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send MQTT connect packet: %w", err)
	}

	pkt, err := packets.ReadPacket(conn)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("read MQTT connack: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})

	connack, ok := pkt.(*packets.ConnackPacket)
	if !ok {
		_ = conn.Close()
		return fmt.Errorf("expected CONNACK, received %T", pkt)
	}
	if connack.ReturnCode != packets.Accepted {
		_ = conn.Close()
		return fmt.Errorf("MQTT connection refused: %s (code %d)", packets.ConnackReturnCodes[connack.ReturnCode], connack.ReturnCode)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected.Store(true)
	c.closed.Store(false)
	c.doneCh = make(chan struct{})
	c.mu.Unlock()

	slog.Info("connected to MQTT broker (pure TCP)", slog.String("broker", addr), slog.String("client_id", cp.ClientIdentifier))

	go c.readLoop(conn)
	go c.keepAliveLoop(keepAlive)

	return nil
}

func (c *TCPClient) readLoop(conn net.Conn) {
	for {
		pkt, err := packets.ReadPacket(conn)
		if err != nil {
			if c.closed.Load() {
				return
			}
			slog.Warn("MQTT TCP connection lost", slog.Any("error", err))
			c.handleDisconnect(conn)
			return
		}

		switch p := pkt.(type) {
		case *packets.PublishPacket:
			if p.Qos == 1 {
				ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
				ack.MessageID = p.MessageID
				_ = c.writePacket(ack)
			}
			c.dispatch(p.TopicName, p.Payload)

		case *packets.PubackPacket:
			c.ackMu.Lock()
			if ch, ok := c.pubacks[p.MessageID]; ok {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
			c.ackMu.Unlock()

		case *packets.SubackPacket:
			c.ackMu.Lock()
			if ch, ok := c.subacks[p.MessageID]; ok {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
			c.ackMu.Unlock()

		case *packets.PingrespPacket:
			// keepalive response received
		}
	}
}

func (c *TCPClient) keepAliveLoop(interval time.Duration) {
	pingInterval := interval / 2
	if pingInterval < 2*time.Second {
		pingInterval = 2 * time.Second
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.doneCh:
			return
		case <-ticker.C:
			if !c.IsConnected() {
				return
			}
			ping := packets.NewControlPacket(packets.Pingreq).(*packets.PingreqPacket)
			if err := c.writePacket(ping); err != nil {
				return
			}
		}
	}
}

func (c *TCPClient) dispatch(topic string, payload []byte) {
	c.mu.RLock()
	var matched []MessageHandler
	for pattern, handlers := range c.subscribers {
		if matchTopic(pattern, topic) {
			matched = append(matched, handlers...)
		}
	}
	c.mu.RUnlock()

	for _, handler := range matched {
		h := handler
		go h(topic, payload)
	}
}

func (c *TCPClient) writePacket(p packets.ControlPacket) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	connected := c.connected.Load()
	c.mu.RUnlock()

	if !connected || conn == nil {
		return ErrNotConnected
	}
	return p.Write(conn)
}

func (c *TCPClient) handleDisconnect(deadConn net.Conn) {
	c.mu.Lock()
	if c.conn == deadConn {
		_ = deadConn.Close()
		c.conn = nil
		c.connected.Store(false)
	}
	c.mu.Unlock()

	if !c.closed.Load() && c.cfg.AutoReconnect {
		go c.reconnectLoop()
	}
}

func (c *TCPClient) reconnectLoop() {
	interval := 2 * time.Second
	maxInterval := c.cfg.MaxReconnectInterval
	if maxInterval <= 0 {
		maxInterval = 60 * time.Second
	}

	for {
		if c.closed.Load() || c.IsConnected() {
			return
		}

		time.Sleep(interval)
		if c.closed.Load() {
			return
		}

		slog.Info("attempting to reconnect to MQTT broker (pure TCP)...", slog.String("broker", c.cfg.Broker))
		if err := c.Connect(); err == nil {
			slog.Info("reconnected to MQTT broker (pure TCP)")
			c.resubscribeAll()
			return
		}

		interval *= 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}

func (c *TCPClient) resubscribeAll() {
	c.mu.RLock()
	topics := make([]string, 0, len(c.subscribers))
	for t := range c.subscribers {
		topics = append(topics, t)
	}
	c.mu.RUnlock()

	for _, topic := range topics {
		sp := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
		sp.MessageID = c.nextMsgID()
		sp.Topics = []string{topic}
		sp.Qoss = []byte{c.cfg.QoS}
		_ = c.writePacket(sp)
	}
}

// Disconnect gracefully disconnects from the MQTT broker.
func (c *TCPClient) Disconnect(quiesce uint) {
	if c.closed.Swap(true) {
		return
	}
	c.connected.Store(false)

	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	if c.doneCh != nil {
		close(c.doneCh)
	}
	c.mu.Unlock()

	if conn != nil {
		dp := packets.NewControlPacket(packets.Disconnect).(*packets.DisconnectPacket)
		_ = dp.Write(conn)
		if quiesce > 0 {
			time.Sleep(time.Duration(quiesce) * time.Millisecond)
		}
		_ = conn.Close()
	}
	slog.Info("disconnected from MQTT broker (pure TCP)")
}

// Publish publishes an MQTT message to the specified topic.
func (c *TCPClient) Publish(topic string, qos byte, retained bool, payload []byte) error {
	slog.Debug("publishing MQTT message (pure TCP)",
		slog.String("topic", topic),
		slog.Int("bytes", len(payload)),
		slog.Int("qos", int(qos)),
		slog.Bool("retained", retained),
	)

	if !c.IsConnected() {
		return ErrNotConnected
	}

	pp := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	pp.TopicName = topic
	pp.Qos = qos
	pp.Retain = retained
	pp.Payload = payload

	if qos == 0 {
		return c.writePacket(pp)
	}

	msgID := c.nextMsgID()
	pp.MessageID = msgID

	ackCh := make(chan struct{}, 1)
	c.ackMu.Lock()
	c.pubacks[msgID] = ackCh
	c.ackMu.Unlock()

	defer func() {
		c.ackMu.Lock()
		delete(c.pubacks, msgID)
		c.ackMu.Unlock()
	}()

	if err := c.writePacket(pp); err != nil {
		return err
	}

	timeout := c.cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	select {
	case <-ackCh:
		return nil
	case <-time.After(timeout):
		return errors.New("MQTT publish timeout")
	}
}

// Subscribe subscribes to a topic pattern and registers a message handler.
func (c *TCPClient) Subscribe(topic string, qos byte, handler MessageHandler) error {
	slog.Debug("subscribing to MQTT topic (pure TCP)", slog.String("topic", topic), slog.Int("qos", int(qos)))

	c.mu.Lock()
	c.subscribers[topic] = append(c.subscribers[topic], handler)
	c.mu.Unlock()

	if !c.IsConnected() {
		return nil
	}

	sp := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
	msgID := c.nextMsgID()
	sp.MessageID = msgID
	sp.Topics = []string{topic}
	sp.Qoss = []byte{qos}

	ackCh := make(chan struct{}, 1)
	c.ackMu.Lock()
	c.subacks[msgID] = ackCh
	c.ackMu.Unlock()

	defer func() {
		c.ackMu.Lock()
		delete(c.subacks, msgID)
		c.ackMu.Unlock()
	}()

	if err := c.writePacket(sp); err != nil {
		return err
	}

	timeout := c.cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	select {
	case <-ackCh:
		return nil
	case <-time.After(timeout):
		return errors.New("MQTT subscribe timeout")
	}
}

// IsConnected returns whether the client is currently connected.
func (c *TCPClient) IsConnected() bool {
	return c.connected.Load()
}

func matchTopic(pattern, topic string) bool {
	if pattern == topic || pattern == "#" {
		return true
	}
	pParts := strings.Split(pattern, "/")
	tParts := strings.Split(topic, "/")
	for i := 0; i < len(pParts); i++ {
		if pParts[i] == "#" {
			return true
		}
		if i >= len(tParts) {
			return false
		}
		if pParts[i] != "+" && pParts[i] != tParts[i] {
			return false
		}
	}
	return len(pParts) == len(tParts)
}

var _ MQTTClient = (*TCPClient)(nil)
