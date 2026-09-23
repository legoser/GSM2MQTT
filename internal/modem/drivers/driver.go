// Package drivers provides specific GSM modem hardware drivers.
package drivers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/modem/at"
)

// ATRunner executes AT commands and returns responses.
type ATRunner interface {
	Send(cmd string, timeout time.Duration) (*at.Response, error)
	SendCommand(ctx context.Context, cmd string) (string, error)
}

// BaseDriver implements common 3GPP AT command operations shared across GSM modems.
type BaseDriver struct {
	runner ATRunner
}

// NewBaseDriver creates a new BaseDriver instance.
func NewBaseDriver(runner ATRunner) *BaseDriver {
	return &BaseDriver{runner: runner}
}

// Init sends the standard AT initialization sequence.
func (d *BaseDriver) Init(ctx context.Context) error {
	initCmds := []string{
		"ATE0",              // Echo off
		"AT+CMEE=2",         // Enable verbose error reporting
		"AT+CMGF=0",         // PDU mode for SMS
		"AT+CNMI=2,1,0,1,0", // New message notifications
		"AT+CLIP=1",         // Enable caller ID presentation
	}

	for _, cmd := range initCmds {
		slog.Debug("driver sending init command", slog.String("cmd", cmd))
		resp, err := d.runner.Send(cmd, 3*time.Second)
		if err != nil {
			return fmt.Errorf("init command %q failed: %w", cmd, err)
		}
		if resp.Error {
			return fmt.Errorf("init command %q returned error: %v", cmd, resp.Lines)
		}
	}

	return nil
}

// Identify returns hardware information about the modem.
func (d *BaseDriver) Identify() (*modem.Info, error) {
	info := &modem.Info{}
	info.Manufacturer = d.queryClean("AT+CGMI")
	info.Model = d.queryClean("AT+CGMM")
	info.Revision = d.queryClean("AT+CGMR")
	info.IMEI = d.queryClean("AT+CGSN")
	info.IMSI = d.queryClean("AT+CIMI")
	return info, nil
}

// Close closes any driver-level state.
func (d *BaseDriver) Close() error {
	return nil
}

// DefaultDialTimeout specifies how long to wait for call setup response from network.
const DefaultDialTimeout = 30 * time.Second

// Dial places an outgoing voice call.
func (d *BaseDriver) Dial(number string) error {
	cleanNum := strings.TrimSpace(number)
	slog.Info("modem dialing voice call", slog.String("number", cleanNum))
	cmd := fmt.Sprintf("ATD%s;", cleanNum)
	err := d.execSimple(cmd, DefaultDialTimeout)
	if err != nil {
		slog.Error("modem voice call dial failed", slog.String("number", cleanNum), slog.Any("error", err))
		// Clean up any half-opened voice call on modem
		_ = d.execSimple("ATH", 3*time.Second)
	}
	return err
}

// Answer answers an incoming voice call.
func (d *BaseDriver) Answer() error {
	slog.Info("modem answering voice call")
	return d.execSimple("ATA", 5*time.Second)
}

// Hangup terminates the current call.
func (d *BaseDriver) Hangup() error {
	slog.Info("modem terminating voice call")
	return d.execSimple("ATH", 5*time.Second)
}

// SendDTMF sends a single DTMF tone during an active call.
func (d *BaseDriver) SendDTMF(digit string) error {
	slog.Debug("modem sending DTMF tone", slog.String("digit", digit))
	cmd := fmt.Sprintf("AT+VTS=%s", strings.TrimSpace(digit))
	return d.execSimple(cmd, 3*time.Second)
}

// SignalQuality queries modem signal strength and returns RSSI (0..31).
func (d *BaseDriver) SignalQuality() (int, error) {
	resp, err := d.runner.Send("AT+CSQ", 3*time.Second)
	if err != nil {
		return 0, err
	}
	if resp.Error || len(resp.Lines) == 0 {
		return 0, fmt.Errorf("AT+CSQ failed: %v", resp.Lines)
	}

	for _, line := range resp.Lines {
		if strings.HasPrefix(line, "+CSQ:") {
			parts := strings.Split(strings.TrimPrefix(line, "+CSQ:"), ",")
			if len(parts) > 0 {
				rssi, err := strconv.Atoi(strings.TrimSpace(parts[0]))
				if err == nil {
					return rssi, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("unable to parse CSQ from %v", resp.Lines)
}

// NetworkRegistration checks the network registration state.
func (d *BaseDriver) NetworkRegistration() (*modem.NetworkStatus, error) {
	resp, err := d.runner.Send("AT+CREG?", 3*time.Second)
	if err == nil && !resp.Error && len(resp.Lines) > 0 {
		if st := parseRegLine(resp.Lines, "+CREG:", "GSM"); st != nil && st.Registered {
			return st, nil
		}
	}

	respG, errG := d.runner.Send("AT+CGREG?", 3*time.Second)
	if errG == nil && !respG.Error && len(respG.Lines) > 0 {
		if st := parseRegLine(respG.Lines, "+CGREG:", "UMTS"); st != nil && st.Registered {
			return st, nil
		}
	}

	if resp != nil && len(resp.Lines) > 0 {
		if st := parseRegLine(resp.Lines, "+CREG:", "GSM"); st != nil {
			return st, nil
		}
	}

	return nil, fmt.Errorf("unable to determine network registration")
}

// OperatorName retrieves the current network operator name.
func (d *BaseDriver) OperatorName() (string, error) {
	resp, err := d.runner.Send("AT+COPS?", 5*time.Second)
	if err != nil {
		return "", err
	}
	for _, line := range resp.Lines {
		if strings.HasPrefix(line, "+COPS:") {
			firstQuote := strings.Index(line, "\"")
			if firstQuote != -1 {
				secondQuote := strings.Index(line[firstQuote+1:], "\"")
				if secondQuote != -1 {
					op := line[firstQuote+1 : firstQuote+1+secondQuote]
					slog.Debug("driver queried operator name", slog.String("operator", op))
					return op, nil
				}
			}
		}
	}
	return "", fmt.Errorf("unable to parse COPS operator from %v", resp.Lines)
}

// SIMStatus checks the status of the SIM card.
func (d *BaseDriver) SIMStatus() (modem.SIMState, error) {
	resp, err := d.runner.Send("AT+CPIN?", 3*time.Second)
	if err != nil {
		return modem.SIMError, err
	}
	for _, line := range resp.Lines {
		if strings.HasPrefix(line, "+CPIN:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "+CPIN:"))
			var simState modem.SIMState
			switch status {
			case "READY":
				simState = modem.SIMReady
			case "SIM PIN":
				simState = modem.SIMPINRequired
			case "SIM PUK":
				simState = modem.SIMPUKRequired
			default:
				simState = modem.SIMState(status)
			}
			slog.Debug("driver queried SIM status", slog.String("status", string(simState)))
			return simState, nil
		}
	}
	return modem.SIMError, fmt.Errorf("unknown CPIN response: %v", resp.Lines)
}

// SendUSSD submits a USSD code request.
func (d *BaseDriver) SendUSSD(code string) (string, error) {
	slog.Debug("driver submitting USSD code", slog.String("code", code))
	cmd := fmt.Sprintf("AT+CUSD=1,%q,15", code)
	resp, err := d.runner.Send(cmd, 10*time.Second)
	if err != nil {
		return "", err
	}
	if resp.Error {
		return "", fmt.Errorf("USSD request failed: %v", resp.Lines)
	}
	return strings.Join(resp.Lines, "\n"), nil
}

// SendRawAT sends an arbitrary AT command.
func (d *BaseDriver) SendRawAT(cmd string) (string, error) {
	slog.Debug("driver sending raw AT command", slog.String("cmd", cmd))
	resp, err := d.runner.Send(cmd, 10*time.Second)
	if err != nil {
		return "", err
	}
	return strings.Join(resp.Lines, "\n"), nil
}

// SendSMS implements modem.SMSSender.
func (d *BaseDriver) SendSMS(number, text string) (byte, error) {
	return 0, fmt.Errorf("SendSMS must be handled via PDU encoder service")
}

// ListSMS implements modem.SMSReader.
func (d *BaseDriver) ListSMS(filter modem.SMSFilter) ([]modem.SMS, error) {
	return nil, nil
}

// DeleteSMS deletes an SMS message by storage index.
func (d *BaseDriver) DeleteSMS(index int) error {
	return d.DeleteMessage(index)
}

func (d *BaseDriver) queryClean(cmd string) string {
	resp, err := d.runner.Send(cmd, 2*time.Second)
	if err != nil || resp.Error || len(resp.Lines) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Lines[0])
}

func (d *BaseDriver) execSimple(cmd string, timeout time.Duration) error {
	resp, err := d.runner.Send(cmd, timeout)
	if err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("command %s failed: %v", cmd, resp.Lines)
	}
	return nil
}
