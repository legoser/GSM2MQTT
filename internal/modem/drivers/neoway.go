package drivers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// NeowayDriver implements driver operations and quirks for Neoway M590/M590E modems.
type NeowayDriver struct {
	*BaseDriver
}

// NewNeowayDriver creates a new NeowayDriver instance.
func NewNeowayDriver(runner ATRunner) *NeowayDriver {
	slog.Debug("neoway driver instance created")
	return &NeowayDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init initializes Neoway M590 modem with standard and vendor-specific parameters.
func (d *NeowayDriver) Init(ctx context.Context) error {
	slog.Debug("starting neoway driver initialization sequence")

	// 1. Send sync AT ping to ensure UART speed alignment
	slog.Debug("sending neoway autobaud sync ping AT")
	_, _ = d.runner.Send("AT", 1*time.Second)

	// 2. Base 3GPP initialization commands
	initCmds := []string{
		"ATE0",              // Echo off
		"AT+CMEE=2",         // Verbose error reporting
		"AT+CMGF=0",         // PDU mode for SMS
		"AT+CNMI=2,1,0,1,0", // New message notifications
		"AT+CLIP=1",         // Enable caller ID presentation
	}

	for _, cmd := range initCmds {
		slog.Debug("executing neoway init command", slog.String("cmd", cmd))
		resp, err := d.runner.Send(cmd, 3*time.Second)
		if err != nil {
			slog.Error("neoway init command communication failed", slog.String("cmd", cmd), slog.Any("error", err))
			return fmt.Errorf("neoway init command %q failed: %w", cmd, err)
		}
		if resp.Error {
			// Some older Neoway firmwares reject CNMI=2,1,0,1,0; fallback to 2,1,0,0,0
			if cmd == "AT+CNMI=2,1,0,1,0" {
				slog.Warn("neoway rejected CNMI=2,1,0,1,0, attempting fallback to CNMI=2,1,0,0,0", slog.Any("response", resp.Lines))
				fallbackResp, fbErr := d.runner.Send("AT+CNMI=2,1,0,0,0", 3*time.Second)
				if fbErr == nil && !fallbackResp.Error {
					slog.Debug("neoway CNMI fallback successful")
					continue
				}
			}
			slog.Error("neoway init command returned error", slog.String("cmd", cmd), slog.Any("response", resp.Lines))
			return fmt.Errorf("neoway init command %q returned error: %v", cmd, resp.Lines)
		}
	}

	slog.Info("neoway M590 driver initialized successfully")
	return nil
}

// Dial initiates an outgoing voice call on Neoway M590.
// Note: M590 is a telemetry module without analog audio path; Dial is used for "call-drop" signaling.
func (d *NeowayDriver) Dial(number string) error {
	cleanNum := strings.TrimSpace(number)
	slog.Info("neoway dialing voice call (call-drop alert)", slog.String("number", cleanNum))

	cmd := fmt.Sprintf("ATD%s;", cleanNum)
	slog.Debug("sending dial command to neoway", slog.String("cmd", cmd))

	err := d.execSimple(cmd, DefaultDialTimeout)
	if err != nil {
		slog.Warn("neoway dial failed or terminated early", slog.String("number", cleanNum), slog.Any("error", err))
		_ = d.execSimple("ATH", 3*time.Second)
		return err
	}

	slog.Debug("neoway dial command accepted by network", slog.String("number", cleanNum))
	return nil
}

// Hangup terminates any active call or outgoing dial attempt.
func (d *NeowayDriver) Hangup() error {
	slog.Info("neoway hanging up voice call")
	err := d.execSimple("ATH", 5*time.Second)
	if err != nil {
		slog.Error("neoway hangup ATH command failed", slog.Any("error", err))
		return err
	}
	slog.Debug("neoway call hung up successfully")
	return nil
}

// BatteryStatus queries power supply voltage via AT+CBC.
func (d *NeowayDriver) BatteryStatus() (*modem.BatteryInfo, error) {
	slog.Debug("querying neoway voltage via AT+CBC")
	resp, err := d.runner.Send("AT+CBC", 3*time.Second)
	if err != nil {
		slog.Error("neoway AT+CBC query failed", slog.Any("error", err))
		return nil, fmt.Errorf("querying AT+CBC failed: %w", err)
	}
	if resp.Error || len(resp.Lines) == 0 {
		slog.Debug("neoway AT+CBC returned error or empty", slog.Any("lines", resp.Lines))
		return nil, fmt.Errorf("AT+CBC returned error: %v", resp.Lines)
	}

	for _, line := range resp.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+CBC:") {
			info, parseErr := parseCBCLine(trimmed)
			if parseErr != nil {
				slog.Warn("failed to parse neoway +CBC response", slog.String("line", trimmed), slog.Any("error", parseErr))
				return nil, parseErr
			}
			slog.Debug("neoway voltage status received",
				slog.Int("millivolts", info.Millivolts),
				slog.Int("percent", info.Percent),
				slog.Bool("charging", info.Charging))
			return info, nil
		}
	}

	return nil, fmt.Errorf("no +CBC response line found in %v", resp.Lines)
}

// SendUSSD submits a USSD code request to Neoway M590 using UCS-2 hex string encoding.
func (d *NeowayDriver) SendUSSD(code string) (string, error) {
	slog.Info("neoway sending USSD request", slog.String("code", code))

	// Reset any existing session
	_, _ = d.runner.Send("AT+CUSD=2", 1*time.Second)

	// Ensure TE character set is UCS2
	_, _ = d.runner.Send("AT+CSCS=\"UCS2\"", 1*time.Second)

	// Encode USSD string to UCS-2 hex
	var ucs2Hex strings.Builder
	for _, r := range code {
		ucs2Hex.WriteString(fmt.Sprintf("%04X", r))
	}
	hexCode := ucs2Hex.String()

	cmd := fmt.Sprintf("AT+CUSD=1,%q,15", hexCode)
	slog.Debug("sending USSD command to neoway", slog.String("cmd", cmd))

	resp, err := d.runner.Send(cmd, 15*time.Second)
	if err == nil && !resp.Error && len(resp.Lines) > 0 {
		slog.Debug("neoway USSD command returned immediate response", slog.Any("lines", resp.Lines))
		return strings.Join(resp.Lines, "\n"), nil
	}

	// Fallback to plain USSD if hex command failed
	slog.Debug("neoway UCS-2 hex USSD failed or empty, attempting fallback to plain code", slog.Any("err", err))
	_, _ = d.runner.Send("AT+CSCS=\"GSM\"", 1*time.Second)
	fallbackCmd := fmt.Sprintf("AT+CUSD=1,%q,15", code)
	fbResp, fbErr := d.runner.Send(fallbackCmd, 15*time.Second)
	if fbErr != nil {
		slog.Error("neoway USSD fallback failed", slog.Any("error", fbErr))
		return "", fbErr
	}
	if fbResp.Error {
		slog.Error("neoway USSD fallback returned error", slog.Any("lines", fbResp.Lines))
		return "", fmt.Errorf("neoway USSD request failed: %v", fbResp.Lines)
	}

	return strings.Join(fbResp.Lines, "\n"), nil
}

var _ modem.Driver = (*NeowayDriver)(nil)
var _ modem.BatteryProvider = (*NeowayDriver)(nil)
var _ modem.USSDSender = (*NeowayDriver)(nil)
