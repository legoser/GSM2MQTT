// Package security provides rate limiting, phone number filtering,
// and AT command sanitization for the GSM gateway.
package security

import (
	"log/slog"
	"strings"
	"unicode"
)

// Filter filters incoming/outgoing phone numbers based on whitelist/blacklist policy.
type Filter struct {
	mode      string
	whitelist map[string]struct{}
	blacklist map[string]struct{}
}

// NewFilter creates a new phone number Filter.
func NewFilter(mode string, whitelist, blacklist []string) *Filter {
	f := &Filter{
		mode:      strings.ToLower(strings.TrimSpace(mode)),
		whitelist: make(map[string]struct{}, len(whitelist)),
		blacklist: make(map[string]struct{}, len(blacklist)),
	}

	for _, num := range whitelist {
		cleaned := normalizeNumber(num)
		if cleaned != "" {
			f.whitelist[cleaned] = struct{}{}
		}
	}

	for _, num := range blacklist {
		cleaned := normalizeNumber(num)
		if cleaned != "" {
			f.blacklist[cleaned] = struct{}{}
		}
	}

	return f
}

// Allowed returns true if the phone number is permitted under the current policy.
func (f *Filter) Allowed(number string) bool {
	return f.Check(number) == nil
}

// Check validates whether the phone number is permitted under the current policy.
// It returns a typed sentinel error if rejected or invalid.
func (f *Filter) Check(number string) error {
	trimmed := strings.TrimSpace(number)
	if trimmed == "" {
		slog.Warn("phone number rejected: empty number", slog.String("mode", f.mode))
		return ErrEmptyPhoneNumber
	}

	if strings.ContainsAny(number, "\x00\r\n") {
		slog.Warn("phone number rejected: dangerous characters", slog.String("number", number), slog.String("mode", f.mode))
		return ErrDangerousChars
	}

	normalized := normalizeNumber(trimmed)

	switch f.mode {
	case "all":
		slog.Debug("phone number allowed (mode: all)", slog.String("number", normalized))
		return nil

	case "whitelist":
		if _, ok := f.whitelist[normalized]; ok {
			slog.Debug("phone number allowed (whitelisted)", slog.String("number", normalized))
			return nil
		}
		slog.Warn("phone number rejected: not whitelisted", slog.String("number", normalized))
		return ErrNumberNotWhitelisted

	case "blacklist":
		if _, ok := f.blacklist[normalized]; ok {
			slog.Warn("phone number rejected: blacklisted", slog.String("number", normalized))
			return ErrNumberBlocked
		}
		slog.Debug("phone number allowed (not blacklisted)", slog.String("number", normalized))
		return nil

	default:
		slog.Warn("phone number rejected: invalid filter mode", slog.String("mode", f.mode))
		return ErrInvalidFilterMode
	}
}

// normalizeNumber strips spaces, parentheses, hyphens, and dots,
// and normalizes alphanumeric senders to uppercase.
func normalizeNumber(number string) string {
	var b strings.Builder
	b.Grow(len(number))

	hasLetters := false
	for _, r := range number {
		if unicode.IsLetter(r) {
			hasLetters = true
			break
		}
	}

	for _, r := range number {
		if r == ' ' || r == '-' || r == '(' || r == ')' || r == '.' {
			continue
		}
		if hasLetters {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}
