package drivers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// SelectStorage configures the active message storage area using AT+CPMS.
func (d *BaseDriver) SelectStorage(mem string) (*modem.StorageStatus, error) {
	cmd := fmt.Sprintf("AT+CPMS=%q,%q,%q", mem, mem, mem)
	resp, err := d.runner.Send(cmd, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("select storage %s failed: %w", mem, err)
	}
	if resp.Error {
		return nil, fmt.Errorf("select storage %s rejected: %v", mem, resp.Lines)
	}

	for _, line := range resp.Lines {
		if st := parseCPMSLine(line, mem); st != nil {
			return st, nil
		}
	}
	return &modem.StorageStatus{Name: mem}, nil
}

// StorageCapacity queries current storage usage using AT+CPMS?.
func (d *BaseDriver) StorageCapacity() (*modem.StorageStatus, error) {
	resp, err := d.runner.Send("AT+CPMS?", 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("query storage capacity failed: %w", err)
	}
	if resp.Error {
		return nil, fmt.Errorf("query storage capacity rejected: %v", resp.Lines)
	}

	for _, line := range resp.Lines {
		if st := parseCPMSLine(line, ""); st != nil {
			return st, nil
		}
	}
	return nil, fmt.Errorf("unable to parse storage capacity from %v", resp.Lines)
}

// ListMessages retrieves all stored messages using AT+CMGL=4 in PDU mode.
func (d *BaseDriver) ListMessages() ([]modem.StoredMessage, error) {
	resp, err := d.runner.Send("AT+CMGL=4", 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("list stored messages failed: %w", err)
	}
	if resp.Error {
		return nil, fmt.Errorf("list stored messages rejected: %v", resp.Lines)
	}

	var msgs []modem.StoredMessage
	for i := 0; i < len(resp.Lines); i++ {
		line := strings.TrimSpace(resp.Lines[i])
		if strings.HasPrefix(line, "+CMGL:") {
			idx, stat := parseCMGLLine(line)
			if idx >= 0 && i+1 < len(resp.Lines) {
				pduHex := strings.TrimSpace(resp.Lines[i+1])
				msgs = append(msgs, modem.StoredMessage{
					Index:  idx,
					Status: stat,
					PDUHex: pduHex,
				})
				i++ // Skip the PDU line
			}
		}
	}
	return msgs, nil
}

// DeleteMessage removes an SMS message from the active storage by index.
func (d *BaseDriver) DeleteMessage(index int) error {
	cmd := fmt.Sprintf("AT+CMGD=%d", index)
	return d.execSimple(cmd, 5*time.Second)
}

func parseCPMSLine(line, fallbackName string) *modem.StorageStatus {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "+CPMS:") {
		return nil
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "+CPMS:"))
	parts := strings.Split(body, ",")
	if len(parts) < 2 {
		return nil
	}

	name := fallbackName
	usedIdx := 0
	totalIdx := 1

	first := strings.Trim(strings.TrimSpace(parts[0]), "\"")
	if _, err := strconv.Atoi(first); err != nil {
		// First part is storage name: e.g. "SM", 15, 15
		name = first
		usedIdx = 1
		totalIdx = 2
	}

	if len(parts) <= totalIdx {
		return nil
	}

	used, _ := strconv.Atoi(strings.TrimSpace(parts[usedIdx]))
	total, _ := strconv.Atoi(strings.TrimSpace(parts[totalIdx]))

	return &modem.StorageStatus{
		Name:  name,
		Used:  used,
		Total: total,
	}
}

func parseCMGLLine(line string) (index, status int) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "+CMGL:"))
	parts := strings.Split(body, ",")
	if len(parts) < 2 {
		return -1, -1
	}
	idx, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	stat, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return -1, -1
	}
	return idx, stat
}
