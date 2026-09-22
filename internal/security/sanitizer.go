package security

// Sanitizer validates and filters raw AT commands to prevent destructive or unauthorized operations.
type Sanitizer struct {
	blockedPrefixes []string
	allowRaw        bool
}

// NewSanitizer creates a new AT command Sanitizer.
func NewSanitizer(allowRaw bool, blockedCommands []string) *Sanitizer {
	return &Sanitizer{
		allowRaw:        allowRaw,
		blockedPrefixes: blockedCommands,
	}
}

// IsAllowed returns true if the given raw AT command is permitted to execute.
// Unimplemented stub for TDD.
func (s *Sanitizer) IsAllowed(cmd string) bool {
	// STUB for TDD: will fail tests
	return false
}
