// Package security provides rate limiting and number filtering for the GSM gateway.
package security

// Filter filters incoming/outgoing phone numbers based on whitelist/blacklist policy.
type Filter struct {
	mode      string
	whitelist map[string]bool
	blacklist map[string]bool
}

// NewFilter creates a new phone number Filter.
func NewFilter(mode string, whitelist, blacklist []string) *Filter {
	return &Filter{
		mode: mode,
	}
}

// Allowed returns true if the phone number is permitted under current policy.
func (f *Filter) Allowed(number string) bool {
	// STUB for TDD: will fail tests
	return false
}
