package drivers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// SiemensDriver implements driver quirks for Siemens/Cinterion TC35, MC35, MC55, TC65 modems.
type SiemensDriver struct {
	*BaseDriver
}

// NewSiemensDriver creates a new SiemensDriver.
func NewSiemensDriver(runner ATRunner) *SiemensDriver {
	return &SiemensDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init synchronizes the auto-baud rate on RS-232 and executes Siemens-specific initialization.
func (d *SiemensDriver) Init(ctx context.Context) error {
	// Ping AT to synchronize baud rate
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := d.runner.Send("AT", 1*time.Second)
		if err == nil && resp.OK {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	initCmds := []string{
		"ATE0",      // Echo off
		"AT+CMEE=2", // Enable verbose error reporting
		"AT+CMGF=0", // PDU mode for SMS
		"AT+CLIP=1", // Enable caller ID presentation
		"AT+CRC=1",  // Extended cellular result codes
	}

	for _, cmd := range initCmds {
		time.Sleep(50 * time.Millisecond)
		resp, err := d.runner.Send(cmd, 3*time.Second)
		if err != nil {
			return fmt.Errorf("siemens init command %q failed: %w", cmd, err)
		}
		if resp.Error {
			return fmt.Errorf("siemens init command %q returned error: %v", cmd, resp.Lines)
		}
	}

	// Siemens TC35/MC35 only supports certain CNMI parameter combinations:
	// Mode 2, mt 1, bm 0, ds 2, bfr 1. If ds=2 is rejected, fall back to ds=0.
	cnmiCandidates := []string{
		"AT+CNMI=2,1,0,2,1",
		"AT+CNMI=2,1,0,0,1",
		"AT+CNMI=1,1,0,0,1",
	}
	var cnmiErr error
	cnmiSuccess := false
	for _, cnmi := range cnmiCandidates {
		resp, err := d.runner.Send(cnmi, 3*time.Second)
		if err == nil && !resp.Error {
			cnmiSuccess = true
			break
		}
		cnmiErr = err
	}
	if !cnmiSuccess && cnmiErr != nil {
		return fmt.Errorf("siemens CNMI setup failed: %w", cnmiErr)
	}

	// Set default SMS storage to SIM card
	_, _ = d.runner.Send("AT+CPMS=\"SM\",\"SM\",\"SM\"", 3*time.Second)

	return nil
}

// Identify returns hardware information about the Siemens modem using ATI and standard registers.
func (d *SiemensDriver) Identify() (*modem.Info, error) {
	info := &modem.Info{}

	// Siemens ATI returns 3 lines: Manufacturer, Model, Revision
	resp, err := d.runner.Send("ATI", 2*time.Second)
	if err == nil && !resp.Error && len(resp.Lines) > 0 {
		for _, line := range resp.Lines {
			clean := strings.TrimSpace(line)
			if clean == "" || clean == "OK" {
				continue
			}
			if info.Manufacturer == "" {
				info.Manufacturer = clean
			} else if info.Model == "" {
				info.Model = clean
			} else if info.Revision == "" {
				info.Revision = clean
			}
		}
	}

	time.Sleep(50 * time.Millisecond)

	if info.Manufacturer == "" {
		info.Manufacturer = d.queryClean("AT+CGMI")
		time.Sleep(50 * time.Millisecond)
	}
	if info.Model == "" {
		info.Model = d.queryClean("AT+CGMM")
		time.Sleep(50 * time.Millisecond)
	}
	if info.Revision == "" {
		info.Revision = d.queryClean("AT+CGMR")
		time.Sleep(50 * time.Millisecond)
	}

	info.IMEI = d.queryClean("AT+CGSN")
	time.Sleep(50 * time.Millisecond)
	info.IMSI = d.queryClean("AT+CIMI")

	return info, nil
}

var _ modem.Driver = (*SiemensDriver)(nil)

