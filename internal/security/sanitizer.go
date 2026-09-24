package security

import (
	"strings"
	"unicode"
)

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

	// Check allowlist
	for _, allowed := range s.allowedPrefixes {
		if strings.HasPrefix(canonicalCmd, allowed) {
			return nil
		}
	}

	return ErrCommandBlocked
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
