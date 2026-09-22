package operator

import (
	"regexp"
	"strconv"
	"strings"
)

var defaultBalancePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:баланс|balance|balans|остаток|schete)[:\s]*([+-]?\d+[.,]?\d*)`),
	regexp.MustCompile(`([+-]?\d+[.,]?\d*)\s*(?:руб|р|rub|usd|eur|₽|\$)`),
}

var debtPattern = regexp.MustCompile(`(?i)(?:задолженност|долг|debt)`)

// ParseBalance extracts the numeric balance amount from operator response text.
func ParseBalance(text string, customRegex string) (float64, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, ErrEmptyResponse
	}

	isDebt := debtPattern.MatchString(trimmed)

	if customRegex != "" {
		re, err := regexp.Compile(customRegex)
		if err != nil {
			return 0, err
		}
		matches := re.FindStringSubmatch(trimmed)
		if len(matches) > 1 {
			val, err := parseNumber(matches[1])
			if err == nil && isDebt && val > 0 {
				val = -val
			}
			return val, err
		}
		return 0, ErrBalanceNotFound
	}

	for _, pattern := range defaultBalancePatterns {
		matches := pattern.FindStringSubmatch(trimmed)
		if len(matches) > 1 {
			val, err := parseNumber(matches[1])
			if err == nil && isDebt && val > 0 {
				val = -val
			}
			return val, err
		}
	}

	return 0, ErrBalanceNotFound
}

func parseNumber(raw string) (float64, error) {
	normalized := strings.ReplaceAll(raw, ",", ".")
	val, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, ErrInvalidBalanceFormat
	}
	return val, nil
}
