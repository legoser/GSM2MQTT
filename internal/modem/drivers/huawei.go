package drivers

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

// HuaweiDriver implements driver quirks for Huawei USB modems.
type HuaweiDriver struct {
	*BaseDriver
}

// NewHuaweiDriver creates a new HuaweiDriver.
func NewHuaweiDriver(runner ATRunner) *HuaweiDriver {
	return &HuaweiDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init initializes the Huawei USB modem with Huawei-specific AT settings and CNMI fallbacks.
func (d *HuaweiDriver) Init(ctx context.Context) error {
	initCmds := []string{
		"ATE0",            // Echo off
		"AT+CMEE=2",       // Verbose error reporting
		"AT+CSCS=\"GSM\"", // Standard GSM charset for SMS & USSD
		"AT+CMGF=0",       // PDU mode for SMS
	}
	for _, cmd := range initCmds {
		_, _ = d.runner.Send(cmd, 3*time.Second)
	}

	// Huawei E1550 rejects AT+CNMI=2,1,0,1,0 with +CMS ERROR: 303.
	// Try fallback variants.
	cnmiVariants := []string{
		"AT+CNMI=2,1,0,0,0",
		"AT+CNMI=1,1,0,0,0",
		"AT+CNMI=2,1,0,1,0",
	}
	for _, cnmi := range cnmiVariants {
		resp, err := d.runner.Send(cnmi, 3*time.Second)
		if err == nil && !resp.Error {
			break
		}
	}

	_, _ = d.runner.Send("AT^CURC=0", 2*time.Second) // Disable proprietary URC spam
	_, _ = d.runner.Send("AT+CLIP=1", 3*time.Second) // Enable caller ID presentation
	return nil
}

// SendUSSD submits a USSD code request with Huawei-specific plain text and 7-bit PDU fallback.
func (d *HuaweiDriver) SendUSSD(code string) (string, error) {
	// 1. Try plain text with DCS 15
	cmd := fmt.Sprintf("AT+CUSD=1,%q,15", code)
	resp, err := d.runner.Send(cmd, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	// 2. Try plain text without DCS
	cmdNoDCS := fmt.Sprintf("AT+CUSD=1,%q", code)
	resp, err = d.runner.Send(cmdNoDCS, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	// 3. Encode into 7-bit GSM packed hex (required by Huawei E1550/E173)
	septets := pdu.EncodeGSM7(code)
	packed := pdu.PackSeptets(septets, 0)
	pduHex := strings.ToUpper(hex.EncodeToString(packed))

	cmdPDU := fmt.Sprintf("AT+CUSD=1,%q,15", pduHex)
	resp, err = d.runner.Send(cmdPDU, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	cmdPDUNoDCS := fmt.Sprintf("AT+CUSD=1,%q", pduHex)
	resp, err = d.runner.Send(cmdPDUNoDCS, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("USSD request failed: %v", resp.Lines)
}

var _ modem.Driver = (*HuaweiDriver)(nil)
