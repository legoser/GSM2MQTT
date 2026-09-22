package security

import "time"

// RateLimiter limits the frequency of outgoing SMS messages to prevent flooding.
type RateLimiter struct {
	maxPerMinute int
	window       time.Duration
}

// NewRateLimiter creates a new SMS RateLimiter.
func NewRateLimiter(maxPerMinute int) *RateLimiter {
	return &RateLimiter{
		maxPerMinute: maxPerMinute,
		window:       time.Minute,
	}
}

// Allow returns true if sending an SMS to the number is permitted right now.
func (r *RateLimiter) Allow(number string) bool {
	// STUB for TDD: will fail tests
	return false
}
