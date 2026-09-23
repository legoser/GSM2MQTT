package at

import (
	"log/slog"
	"strings"
)

func (e *Engine) dispatchURC(line string) {
	slog.Debug("AT URC received", slog.String("line", line))
	select {
	case e.urcChan <- line:
	default:
		slog.Warn("AT URC buffer full, notification dropped", slog.String("line", line))
	}
}

// isCallTerminationResponse checks if a response line indicates call setup termination.
func isCallTerminationResponse(cmd, line string) bool {
	upperCmd := strings.ToUpper(strings.TrimSpace(cmd))
	if strings.HasPrefix(upperCmd, "ATD") || upperCmd == "ATA" || upperCmd == "ATH" {
		switch line {
		case "NO CARRIER", "BUSY", "NO ANSWER", "NO DIALTONE":
			return true
		}
	}
	return false
}

// isCommandResponse checks if the incoming line corresponds to the command in flight.
func isCommandResponse(cmd, line string) bool {
	upperCmd := strings.ToUpper(strings.TrimSpace(cmd))
	upperLine := strings.ToUpper(strings.TrimSpace(line))

	if strings.HasPrefix(upperCmd, "AT") {
		clean := strings.TrimPrefix(upperCmd, "AT")
		for _, stop := range []string{"?", "=", "\r", "\n"} {
			if idx := strings.Index(clean, stop); idx >= 0 {
				clean = clean[:idx]
			}
		}
		if clean != "" && strings.HasPrefix(upperLine, clean+":") {
			return true
		}
	}
	return false
}

// isURC checks if an unsolicited result code prefix or known modem event is matched.
func isURC(line string) bool {
	urcPrefixes := []string{
		"+CLIP:", "+CMTI:", "+CMT:", "+CDS:", "+DTMF:",
		"+CUSD:", "RING", "+CRING:", "+CREG:",
		"+CGREG:", "+CEREG:", "NO CARRIER", "+COLP:",
		"^ORIG:", "^CONN:", "^CEND:", "^RSSI:", "^MODE:",
		"^DSFLOWRPT:", "^BOOT:", "^SRVST:",
		// SIMCom SIM800 / SIM900 hardware event lines
		"RDY", "Call Ready", "SMS Ready", "NORMAL POWER DOWN",
		"UNDER-VOLTAGE WARNNING", "UNDER-VOLTAGE POWER DOWN",
		"OVER-VOLTAGE WARNNING", "OVER-VOLTAGE POWER DOWN",
		// Neoway M590 / M590E hardware event lines
		"MODEM:STARTUP", "+PBREADY", "+ZUSIMR:",
	}
	for _, p := range urcPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}
