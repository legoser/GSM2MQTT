package security

import (
	"strings"
	"unicode"
)

// Sanitizer validates raw AT commands against an allowlist to prevent
// destructive or unauthorized operations. Allowlist entries are matched
// on token boundaries so a broad entry like "AT+C" can never authorize
// "AT+CFUN" or "AT+CPIN".
type Sanitizer struct {
	allowRaw        bool
	allowedPrefixes []string
}

// NewSanitizer creates a new AT command Sanitizer.
func NewSanitizer(allowRaw bool, allowedCommands []string) *Sanitizer {
	normalizedAllowed := make([]string, 0, len(allowedCommands))
	for _, cmd := range allowedCommands {
		cleaned := canonicalizeCommand(cmd)
		if cleaned != "" {
			normalizedAllowed = append(normalizedAllowed, cleaned)
		}
	}

	return &Sanitizer{
		allowRaw:        allowRaw,
		allowedPrefixes: normalizedAllowed,
	}
}

// IsAllowed returns true if the given raw AT command is permitted to execute.
func (s *Sanitizer) IsAllowed(cmd string) bool {
	return s.Validate(cmd) == nil
}

// Validate checks whether the raw AT command is permitted to execute.
// It returns a typed sentinel error if rejected or invalid.
func (s *Sanitizer) Validate(cmd string) error {
	if strings.ContainsAny(cmd, "\x00\x1A\x1B\x07\x0B\x0C\x7F\r\n") {
		return ErrDangerousChars
	}

	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return ErrEmptyCommand
	}

	if !s.allowRaw {
		return ErrRawATDisabled
	}

	canonicalCmd := canonicalizeCommand(trimmed)

	// Check allowlist on token boundaries: exact match or the next
	// character must terminate the token (?, =, ;, comma, quote, slash
	// or digit for indexed commands like ATI0). This prevents "AT+C"
	// from authorizing "AT+CFUN", "AT+CPIN" or "AT+CMGS".
	for _, allowed := range s.allowedPrefixes {
		if matchesAllowed(canonicalCmd, allowed) {
			return nil
		}
	}

	return ErrCommandBlocked
}

// matchesAllowed reports whether canonicalCmd starts with the allowlist
// entry on a token boundary.
func matchesAllowed(canonicalCmd, allowed string) bool {
	if canonicalCmd == allowed {
		return true
	}
	if !strings.HasPrefix(canonicalCmd, allowed) {
		return false
	}
	next := canonicalCmd[len(allowed)]
	switch next {
	case '?', '=', ';', ',', '"', '/', ':', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}

// canonicalizeCommand removes all whitespace and converts to uppercase for reliable prefix comparison.
func canonicalizeCommand(cmd string) string {
	var b strings.Builder
	b.Grow(len(cmd))

	for _, r := range cmd {
		if !unicode.IsSpace(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}

	return b.String()
}
