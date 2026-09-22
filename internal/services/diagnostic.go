package services

import (
	"context"
	"fmt"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

// DiagnosticReport contains detailed post-failure self-diagnostic results.
type DiagnosticReport struct {
	Timestamp time.Time `json:"timestamp"`
	ModemID   string    `json:"modem_id"`
	Trigger   string    `json:"trigger"`
	SIMReady  bool      `json:"sim_ready"`
	SIMState  string    `json:"sim_state"`
	NetworkOK bool      `json:"network_ok"`
	Network   string    `json:"network"`
	SignalOK  bool      `json:"signal_ok"`
	RSSI      int       `json:"rssi"`
	SignalDBm int       `json:"signal_dbm"`
	BalanceOK bool      `json:"balance_ok"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	Issue     string    `json:"issue"`
	Advice    string    `json:"advice"`
}

// BalanceQueryFunc queries the latest monetary account balance.
type BalanceQueryFunc func() (float64, string, error)

// DiagnosticService runs multi-stage fault tree analysis upon modem or transmission errors.
type DiagnosticService struct {
	modemID      string
	provider     modem.StatusProvider
	balanceQuery BalanceQueryFunc
	onAlert      func(alert string)
}

// NewDiagnosticService creates a new DiagnosticService.
func NewDiagnosticService(
	modemID string,
	provider modem.StatusProvider,
	balanceQuery BalanceQueryFunc,
	onAlert func(alert string),
) *DiagnosticService {
	return &DiagnosticService{
		modemID:      modemID,
		provider:     provider,
		balanceQuery: balanceQuery,
		onAlert:      onAlert,
	}
}

// RunDiagnostic executes the sequential diagnostic pipeline and produces a report.
func (d *DiagnosticService) RunDiagnostic(ctx context.Context, trigger string) (*DiagnosticReport, error) {
	report := &DiagnosticReport{
		Timestamp: time.Now(),
		ModemID:   d.modemID,
		Trigger:   trigger,
		BalanceOK: true,
		Currency:  tariff.DefaultCurrency,
	}

	d.checkSIM(report)
	d.checkNetwork(report)
	d.checkSignal(report)
	d.checkBalance(report)
	d.synthesizeDiagnosis(report)

	if report.Issue != "" && d.onAlert != nil {
		d.onAlert(fmt.Sprintf("%s: %s (%s)", d.modemID, report.Issue, report.Advice))
	}

	return report, nil
}

func (d *DiagnosticService) checkSIM(report *DiagnosticReport) {
	sim, _ := d.provider.SIMStatus()
	report.SIMState = string(sim)
	report.SIMReady = sim == modem.SIMReady
}

func (d *DiagnosticService) checkNetwork(report *DiagnosticReport) {
	reg, _ := d.provider.NetworkRegistration()
	if reg != nil {
		report.NetworkOK = reg.Registered
		report.Network = reg.Technology
		if reg.Roaming {
			report.Network += " (roaming)"
		}
	} else {
		report.Network = "none"
	}
}

func (d *DiagnosticService) checkSignal(report *DiagnosticReport) {
	rssi, _ := d.provider.SignalQuality()
	report.RSSI = rssi
	report.SignalDBm = calculateDBm(rssi)
	report.SignalOK = rssi >= 5 && rssi != 99
}

func (d *DiagnosticService) checkBalance(report *DiagnosticReport) {
	if d.balanceQuery != nil {
		bal, cur, err := d.balanceQuery()
		if err == nil {
			report.Balance = bal
			if cur != "" {
				report.Currency = cur
			}
			report.BalanceOK = bal > 0.0
		}
	}
}

func (d *DiagnosticService) synthesizeDiagnosis(report *DiagnosticReport) {
	switch {
	case !report.SIMReady:
		report.Issue = fmt.Sprintf("SIM card inaccessible or locked (%s)", report.SIMState)
		report.Advice = "Check SIM card installation, inspect PIN/PUK requirements"
	case !report.NetworkOK:
		report.Issue = "Modem not registered on cellular network"
		report.Advice = "Check external antenna connection or verify carrier coverage"
	case !report.SignalOK:
		report.Issue = fmt.Sprintf("Cellular signal is too weak or noisy (RSSI %d)", report.RSSI)
		report.Advice = "Reposition antenna or check for electromagnetic interference"
	case !report.BalanceOK:
		report.Issue = fmt.Sprintf("Account balance is exhausted or negative (%.2f %s)", report.Balance, report.Currency)
		report.Advice = "Top up SIM card balance with the mobile network operator"
	default:
		report.Issue = "Cellular network / SMSC service center issue"
		report.Advice = "Modem hardware and connection appear healthy; carrier SMS center may be overloaded"
	}
}
