package services

import (
	"context"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

type mockStatusProvider struct {
	rssi      int
	regStatus *modem.NetworkStatus
	operator  string
	simState  modem.SIMState
	err       error
}

func (m *mockStatusProvider) SignalQuality() (int, error) {
	return m.rssi, m.err
}

func (m *mockStatusProvider) NetworkRegistration() (*modem.NetworkStatus, error) {
	return m.regStatus, m.err
}

func (m *mockStatusProvider) OperatorName() (string, error) {
	return m.operator, m.err
}

func (m *mockStatusProvider) SIMStatus() (modem.SIMState, error) {
	return m.simState, m.err
}

func TestStatusService_Poll_Ready(t *testing.T) {
	mock := &mockStatusProvider{
		rssi:      22,
		regStatus: &modem.NetworkStatus{Registered: true, Roaming: false, Technology: "GSM"},
		operator:  "MTS",
		simState:  modem.SIMReady,
	}

	var notifiedHealth *ModemHealth
	var notifiedSignal int

	svc := NewStatusService(StatusServiceConfig{
		ModemID:  "siemens_tc35",
		Interval: time.Minute,
	}, mock, func(rssi int, dbm int) {
		notifiedSignal = rssi
	}, func(h ModemHealth) {
		notifiedHealth = &h
	})

	health, err := svc.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}

	if health.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", health.Status)
	}
	if health.Signal != 22 {
		t.Errorf("expected signal 22, got %d", health.Signal)
	}
	if health.SignalDBm != -69 { // -113 + 2*22 = -69 dBm
		t.Errorf("expected dBm -69, got %d", health.SignalDBm)
	}
	if health.Operator != "MTS" {
		t.Errorf("expected operator 'MTS', got %q", health.Operator)
	}
	if notifiedHealth == nil || notifiedHealth.Status != "ready" {
		t.Errorf("expected health notification")
	}
	if notifiedSignal != 22 {
		t.Errorf("expected signal notification 22, got %d", notifiedSignal)
	}
}

func TestStatusService_Poll_Degraded(t *testing.T) {
	mock := &mockStatusProvider{
		rssi:      4, // Low signal
		regStatus: &modem.NetworkStatus{Registered: true},
		operator:  "MTS",
		simState:  modem.SIMReady,
	}

	svc := NewStatusService(StatusServiceConfig{ModemID: "siemens_tc35"}, mock, nil, nil)
	health, err := svc.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if health.Status != "degraded" {
		t.Errorf("expected status 'degraded' for low signal, got %q", health.Status)
	}
}

func TestStatusService_Poll_SIMNotReady(t *testing.T) {
	mock := &mockStatusProvider{
		rssi:      20,
		regStatus: &modem.NetworkStatus{Registered: false},
		simState:  modem.SIMPINRequired,
	}

	svc := NewStatusService(StatusServiceConfig{ModemID: "siemens_tc35"}, mock, nil, nil)
	health, err := svc.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if health.Status != "not_ready" {
		t.Errorf("expected status 'not_ready' for SIM PIN, got %q", health.Status)
	}
}
