package pool

import (
	"strings"
)

// Strategy defines the load balancing and modem selection strategy.
type Strategy string

const (
	// StrategyRoundRobin distributes requests sequentially across ready modems.
	StrategyRoundRobin Strategy = "round-robin"
	// StrategyFailover always picks the first ready modem in registered priority order.
	StrategyFailover Strategy = "failover"
	// StrategyBestSignal picks the ready modem with the highest signal strength.
	StrategyBestSignal Strategy = "best-signal"
	// StrategyOperatorMatch picks the modem matching the destination operator prefix.
	StrategyOperatorMatch Strategy = "operator-match"
)

// SelectionCriteria contains optional routing hints for modem selection.
type SelectionCriteria struct {
	RecipientNumber string
	RequiredModemID string
}

// IsValid checks if the strategy name is supported.
func (s Strategy) IsValid() bool {
	switch s {
	case StrategyRoundRobin, StrategyFailover, StrategyBestSignal, StrategyOperatorMatch:
		return true
	default:
		return false
	}
}

// pickBestSignal returns the member with highest signal strength from ready list.
func pickBestSignal(ready []ModemMember) ModemMember {
	if len(ready) == 0 {
		return nil
	}
	best := ready[0]
	for _, m := range ready[1:] {
		if m.Signal() > best.Signal() {
			best = m
		}
	}
	return best
}

// pickOperatorMatch attempts to match destination phone number prefix to operator name.
func pickOperatorMatch(ready []ModemMember, number string) ModemMember {
	targetOp := detectOperatorByNumber(number)
	if targetOp != "" {
		for _, m := range ready {
			if strings.EqualFold(m.Operator(), targetOp) {
				return m
			}
		}
	}
	return ready[0]
}

func detectOperatorByNumber(number string) string {
	digits := extractDigits(number)
	if len(digits) < 4 {
		return ""
	}
	// Trim country code 7 or 8
	prefix3 := ""
	if (strings.HasPrefix(digits, "7") || strings.HasPrefix(digits, "8")) && len(digits) >= 4 {
		prefix3 = digits[1:4]
	} else if len(digits) >= 3 {
		prefix3 = digits[:3]
	}

	switch {
	case isMTSPrefix(prefix3):
		return "MTS"
	case isMegaFonPrefix(prefix3):
		return "MegaFon"
	case isBeelinePrefix(prefix3):
		return "Beeline"
	case isTele2Prefix(prefix3):
		return "Tele2"
	default:
		return ""
	}
}

func isMTSPrefix(p string) bool {
	return strings.HasPrefix(p, "91") || strings.HasPrefix(p, "98")
}

func isMegaFonPrefix(p string) bool {
	return strings.HasPrefix(p, "92") || strings.HasPrefix(p, "93")
}

func isBeelinePrefix(p string) bool {
	return strings.HasPrefix(p, "96") || p == "903" || p == "905" || p == "906" || p == "909"
}

func isTele2Prefix(p string) bool {
	return strings.HasPrefix(p, "95") || strings.HasPrefix(p, "99") || p == "977" || p == "900" || p == "901" || p == "902" || p == "904" || p == "908"
}

func extractDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
