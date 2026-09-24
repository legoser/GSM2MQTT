package operator

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var defaultBalancePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:баланс|balance|balans|остаток|schete)[:\s]*([+-]?\d+[.,]?\d*)`),
	regexp.MustCompile(`([+-]?\d+[.,]?\d*)\s*(?:руб|р|rub|usd|eur|₽|\$)`),
}

var debtPattern = regexp.MustCompile(`(?i)(?:задолженност|долг|debt)`)

// customRegexCache memoizes compiled custom balance patterns so a pattern
// from config is compiled once, not on every SMS/USSD response. It also
// bounds ReDoS exposure: a pathologically slow pattern fails once at
// compile or is reused without repeated compile cost.
var customRegexCache sync.Map // map[string]*regexp.Regexp

// compileCustomRegex returns a cached compiled pattern or compiles and
// caches it. Patterns longer than 512 bytes are rejected outright.
func compileCustomRegex(pattern string) (*regexp.Regexp, error) {
	if v, ok := customRegexCache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}
	if len(pattern) > 512 {
		return nil, ErrInvalidBalanceRegex
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := customRegexCache.LoadOrStore(pattern, re)
	return actual.(*regexp.Regexp), nil
}

// ParseBalance extracts the numeric balance amount from operator response text.
func ParseBalance(text string, customRegex string) (float64, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, ErrEmptyResponse
	}

	isDebt := debtPattern.MatchString(trimmed)

	if customRegex != "" {
		re, err := compileCustomRegex(customRegex)
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
