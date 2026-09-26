//go:build no_tls

package mqtt

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

// mockBroker is a minimal TCP server that handles MQTT handshake and packets.
type mockBroker struct {
	listener net.Listener
	addr     string
	mu       sync.Mutex
	conns    []net.Conn
}

func startMockBroker(t *testing.T) *mockBroker {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	b := &mockBroker{
		listener: l,
		addr:     l.Addr().String(),
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			b.mu.Lock()
			b.conns = append(b.conns, conn)
			b.mu.Unlock()
			go b.handleConn(conn)
		}
	}()

	return b
}

func (b *mockBroker) handleConn(conn net.Conn) {
	defer conn.Close()
	for {
		pkt, err := packets.ReadPacket(conn)
		if err != nil {
			return
		}

		switch p := pkt.(type) {
		case *packets.ConnectPacket:
			ca := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
			ca.ReturnCode = packets.Accepted
			_ = ca.Write(conn)

		case *packets.PublishPacket:
			if p.Qos == 1 {
				ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
				ack.MessageID = p.MessageID
				_ = ack.Write(conn)
			}

		case *packets.SubscribePacket:
			sa := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
			sa.MessageID = p.MessageID
			sa.ReturnCodes = make([]byte, len(p.Topics))
			for i := range sa.ReturnCodes {
				sa.ReturnCodes[i] = p.Qoss[i]
			}
			_ = sa.Write(conn)

		case *packets.PingreqPacket:
			pr := packets.NewControlPacket(packets.Pingresp).(*packets.PingrespPacket)
			_ = pr.Write(conn)

		case *packets.DisconnectPacket:
			return
		}
	}
}

func (b *mockBroker) close() {
	_ = b.listener.Close()
	b.mu.Lock()
	for _, c := range b.conns {
		_ = c.Close()
	}
	b.mu.Unlock()
}

func TestTCPClient_Lifecycle(t *testing.T) {
	broker := startMockBroker(t)
	defer broker.close()

	cfg := ClientConfig{
		Broker:         "tcp://" + broker.addr,
		ClientID:       "test-client",
		CleanSession:   true,
		KeepAlive:      5 * time.Second,
		ConnectTimeout: 2 * time.Second,
	}

	client, err := NewTCPClient(cfg)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	if client.IsConnected() {
		t.Fatal("expected IsConnected to be false before Connect()")
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !client.IsConnected() {
		t.Fatal("expected IsConnected to be true after Connect()")
	}

	// Subscribe
	received := make(chan string, 1)
	err = client.Subscribe("test/topic", 1, func(topic string, payload []byte) {
		received <- string(payload)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish QoS 0
	if err := client.Publish("test/topic", 0, false, []byte("qos0-msg")); err != nil {
		t.Fatalf("Publish QoS 0 failed: %v", err)
	}

	// Publish QoS 1
	if err := client.Publish("test/topic", 1, false, []byte("qos1-msg")); err != nil {
		t.Fatalf("Publish QoS 1 failed: %v", err)
	}

	client.Disconnect(10)
	if client.IsConnected() {
		t.Fatal("expected IsConnected to be false after Disconnect()")
	}
}
