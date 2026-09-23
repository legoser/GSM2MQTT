package drivers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// SIMComDriver implements driver quirks for SIMCom SIM800/SIM900 modems.
type SIMComDriver struct {
	*BaseDriver
}

// NewSIMComDriver creates a new SIMComDriver.
func NewSIMComDriver(runner ATRunner) *SIMComDriver {
	return &SIMComDriver{
		BaseDriver: NewBaseDriver(runner),
	}
}

// Init configures sleep mode, full radio functionality, and initializes base parameters.
func (d *SIMComDriver) Init(ctx context.Context) error {
	// Sync autobaud and wake up UART
	_, _ = d.runner.Send("AT", 1*time.Second)

	// Disable sleep mode on UART
	_, _ = d.runner.Send("AT+CSCLK=0", 1*time.Second)

	// Set full phone functionality
	_, _ = d.runner.Send("AT+CFUN=1", 1*time.Second)

	if err := d.BaseDriver.Init(ctx); err != nil {
		return fmt.Errorf("simcom init failed: %w", err)
	}

	return nil
}

// BatteryStatus queries the battery charging state, percentage, and voltage in millivolts via AT+CBC.
func (d *SIMComDriver) BatteryStatus() (*modem.BatteryInfo, error) {
	resp, err := d.runner.Send("AT+CBC", 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("querying AT+CBC failed: %w", err)
	}
	if resp.Error || len(resp.Lines) == 0 {
		return nil, fmt.Errorf("AT+CBC returned error: %v", resp.Lines)
	}

	for _, line := range resp.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+CBC:") {
			return parseCBCLine(trimmed)
		}
	}

	return nil, fmt.Errorf("no +CBC response line found in %v", resp.Lines)
}

func parseCBCLine(line string) (*modem.BatteryInfo, error) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "+CBC:"))
	parts := strings.Split(body, ",")
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed +CBC line: %q", line)
	}

	bcs, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid BCS in +CBC: %w", err)
	}

	bcl, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid BCL in +CBC: %w", err)
	}

	bcv, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return nil, fmt.Errorf("invalid BCV in +CBC: %w", err)
	}

	return &modem.BatteryInfo{
		Charging:   bcs == 1,
		Percent:    bcl,
		Millivolts: bcv,
	}, nil
}

// SetVolume adjusts speaker volume on SIM800 (range 0..100).
func (d *SIMComDriver) SetVolume(level int) error {
	if level < 0 || level > 100 {
		return fmt.Errorf("volume level out of range [0..100]: %d", level)
	}
	return d.execSimple(fmt.Sprintf("AT+CLVL=%d", level), 3*time.Second)
}

// SetMicGain adjusts microphone gain on main channel (range 0..15).
func (d *SIMComDriver) SetMicGain(gain int) error {
	if gain < 0 || gain > 15 {
		return fmt.Errorf("mic gain out of range [0..15]: %d", gain)
	}
	return d.execSimple(fmt.Sprintf("AT+CMIC=0,%d", gain), 3*time.Second)
}

var _ modem.Driver = (*SIMComDriver)(nil)
var _ modem.BatteryProvider = (*SIMComDriver)(nil)
