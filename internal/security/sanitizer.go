package security

import (
	"strings"
	"unicode"
)

// Sanitizer validates and filters raw AT commands to prevent destructive or unauthorized operations.
type Sanitizer struct {
	allowRaw        bool
	blockedPrefixes []string
}

// NewSanitizer creates a new AT command Sanitizer.
func NewSanitizer(allowRaw bool, blockedCommands []string) *Sanitizer {
	normalizedBlocked := make([]string, 0, len(blockedCommands))
	for _, cmd := range blockedCommands {
		cleaned := canonicalizeCommand(cmd)
		if cleaned != "" {
			normalizedBlocked = append(normalizedBlocked, cleaned)
		}
	}

	return &Sanitizer{
		allowRaw:        allowRaw,
		blockedPrefixes: normalizedBlocked,
	}
}

// IsAllowed returns true if the given raw AT command is permitted to execute.
func (s *Sanitizer) IsAllowed(cmd string) bool {
	return s.Validate(cmd) == nil
}

// Validate checks whether the raw AT command is permitted to execute.
// It returns a typed sentinel error if rejected or invalid.
func (s *Sanitizer) Validate(cmd string) error {
	if strings.ContainsAny(cmd, "\x00\x1A\r\n") {
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
	for _, blocked := range s.blockedPrefixes {
		if strings.HasPrefix(canonicalCmd, blocked) {
			return ErrCommandBlocked
		}
	}

	return nil
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
