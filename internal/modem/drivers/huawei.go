package drivers

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
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

	_, _ = d.runner.Send("AT^CURC=1", 2*time.Second)   // Enable URC notifications on PCUI port
	_, _ = d.runner.Send("AT+CLIP=1", 3*time.Second)   // Enable caller ID presentation
	_, _ = d.runner.Send("AT^CVOICE=0", 2*time.Second) // Enable digital voice calls in Huawei firmware
	_, _ = d.runner.Send("AT+COLP=1", 2*time.Second)   // Enable connected line identification
	return nil
}

// SendUSSD submits a USSD code request with Huawei-specific 7-bit PDU encoding first, then plain text fallback.
func (d *HuaweiDriver) SendUSSD(code string) (string, error) {
	// 0. Terminate any dangling previous USSD session
	_, _ = d.runner.Send("AT+CUSD=2", 2*time.Second)

	// 1. Encode into 7-bit GSM packed hex (native requirement for Huawei E1550/E173)
	// USSD codes are typically ASCII-compatible with GSM-7 basic character set.
	packed := packUSSD(code)
	pduHex := strings.ToUpper(hex.EncodeToString(packed))

	cmdPDU := fmt.Sprintf("AT+CUSD=1,%q,15", pduHex)
	resp, err := d.runner.Send(cmdPDU, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	cmdPDUNoDCS := fmt.Sprintf("AT+CUSD=1,%q", pduHex)
	resp, err = d.runner.Send(cmdPDUNoDCS, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	// 2. Fallback to plain text with DCS 15
	cmd := fmt.Sprintf("AT+CUSD=1,%q,15", code)
	resp, err = d.runner.Send(cmd, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	// 3. Fallback to plain text without DCS
	cmdNoDCS := fmt.Sprintf("AT+CUSD=1,%q", code)
	resp, err = d.runner.Send(cmdNoDCS, 10*time.Second)
	if err == nil && !resp.Error {
		return strings.Join(resp.Lines, "\n"), nil
	}

	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("USSD request failed: %v", resp.Lines)
}

// Dial places an outgoing voice call, verifying first if voice is supported by the firmware.
func (d *HuaweiDriver) Dial(number string) error {
	resp, err := d.runner.Send("AT^CVOICE?", 2*time.Second)
	if err == nil && !resp.Error && len(resp.Lines) > 0 {
		line := strings.Join(resp.Lines, " ")
		if strings.Contains(line, "^CVOICE:1") || strings.Contains(line, "^CVOICE:(1)") {
			return fmt.Errorf("voice calls are not supported by this modem firmware (CVOICE=1)")
		}
	}
	return d.BaseDriver.Dial(number)
}

var _ modem.Driver = (*HuaweiDriver)(nil)

// packUSSD converts an ASCII string to 7-bit septets and packs them into 8-bit octets.
func packUSSD(s string) []byte {
	septets := []byte(s) // For USSD (*100#), ASCII maps directly to GSM-7 basic
	if len(septets) == 0 {
		return nil
	}
	numOctets := (len(septets)*7 + 7) / 8
	octets := make([]byte, numOctets)
	bitPos := 0
	for _, s := range septets {
		val := uint16(s & 0x7F)
		byteIdx := bitPos / 8
		bitOffset := bitPos % 8
		octets[byteIdx] |= byte(val << bitOffset)
		if byteIdx+1 < numOctets {
			octets[byteIdx+1] |= byte(val >> (8 - bitOffset))
		}
		bitPos += 7
	}
	return octets
}
