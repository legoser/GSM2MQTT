package services

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

func TestDiagnostic_SIMError(t *testing.T) {
	mock := &mockStatusProvider{
		simState: modem.SIMPINRequired,
	}

	var alertEmitted string
	var mu sync.Mutex

	diag := NewDiagnosticService("siemens_tc35", mock, nil, func(alert string) {
		mu.Lock()
		defer mu.Unlock()
		alertEmitted = alert
	})

	report, err := diag.RunDiagnostic(context.Background(), "sms_send_failure")
	if err != nil {
		t.Fatalf("RunDiagnostic error: %v", err)
	}

	if report.SIMReady {
		t.Errorf("expected SIMReady = false")
	}
	if !strings.Contains(report.Issue, "SIM") {
		t.Errorf("expected issue to mention SIM, got %q", report.Issue)
	}
	mu.Lock()
	if !strings.Contains(alertEmitted, "SIM") {
		t.Errorf("expected alert to mention SIM, got %q", alertEmitted)
	}
	mu.Unlock()
}

func TestDiagnostic_NetworkError(t *testing.T) {
	mock := &mockStatusProvider{
		simState:  modem.SIMReady,
		regStatus: &modem.NetworkStatus{Registered: false},
	}

	diag := NewDiagnosticService("siemens_tc35", mock, nil, nil)
	report, err := diag.RunDiagnostic(context.Background(), "sms_send_failure")
	if err != nil {
		t.Fatalf("RunDiagnostic error: %v", err)
	}

	if report.NetworkOK {
		t.Errorf("expected NetworkOK = false")
	}
	if !strings.Contains(report.Issue, "network") && !strings.Contains(report.Issue, "registered") {
		t.Errorf("expected issue to mention network, got %q", report.Issue)
	}
}

func TestDiagnostic_WeakSignal(t *testing.T) {
	mock := &mockStatusProvider{
		simState:  modem.SIMReady,
		regStatus: &modem.NetworkStatus{Registered: true},
		rssi:      3, // Weak signal
	}

	diag := NewDiagnosticService("siemens_tc35", mock, nil, nil)
	report, err := diag.RunDiagnostic(context.Background(), "sms_send_failure")
	if err != nil {
		t.Fatalf("RunDiagnostic error: %v", err)
	}

	if report.SignalOK {
		t.Errorf("expected SignalOK = false")
	}
	if !strings.Contains(report.Issue, "signal") {
		t.Errorf("expected issue to mention signal, got %q", report.Issue)
	}
}

func TestDiagnostic_ExhaustedBalance(t *testing.T) {
	mock := &mockStatusProvider{
		simState:  modem.SIMReady,
		regStatus: &modem.NetworkStatus{Registered: true},
		rssi:      20,
	}

	balanceQuery := func() (float64, string, error) {
		return 0.0, "RUB", nil
	}

	diag := NewDiagnosticService("siemens_tc35", mock, balanceQuery, nil)
	report, err := diag.RunDiagnostic(context.Background(), "sms_send_failure")
	if err != nil {
		t.Fatalf("RunDiagnostic error: %v", err)
	}

	if report.BalanceOK {
		t.Errorf("expected BalanceOK = false")
	}
	if !strings.Contains(report.Issue, "balance") {
		t.Errorf("expected issue to mention balance, got %q", report.Issue)
	}
}

func TestDiagnostic_CarrierSMSCError(t *testing.T) {
	mock := &mockStatusProvider{
		simState:  modem.SIMReady,
		regStatus: &modem.NetworkStatus{Registered: true},
		rssi:      25,
	}

	balanceQuery := func() (float64, string, error) {
		return 150.0, "RUB", nil
	}

	diag := NewDiagnosticService("siemens_tc35", mock, balanceQuery, nil)
	report, err := diag.RunDiagnostic(context.Background(), "sms_send_failure")
	if err != nil {
		t.Fatalf("RunDiagnostic error: %v", err)
	}

	if !report.SIMReady || !report.NetworkOK || !report.SignalOK || !report.BalanceOK {
		t.Errorf("expected all hardware checks to pass")
	}
	if !strings.Contains(report.Issue, "SMSC") && !strings.Contains(report.Issue, "carrier") {
		t.Errorf("expected issue to mention carrier/SMSC, got %q", report.Issue)
	}
}
