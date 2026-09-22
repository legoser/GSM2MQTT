package sms

import (
	"strings"
	"testing"
)

// FuzzNormalizePhoneNumber uses Go native fuzzing to generate thousands of random/mutated phone numbers.
// Invariants verified:
// 1. Must never panic.
// 2. If no error is returned, the output must either be an E.164 number (starts with '+', 3..16 chars)
//    or a short code (2..6 digits).
func FuzzNormalizePhoneNumber(f *testing.F) {
	// Seed corpus with known edge cases
	seeds := []string{
		"+79001234567",
		"89001234567",
		"8 (900) 123-45-67",
		"+1 (555) 019-2834",
		"112",
		"900",
		"",
		"   ",
		"1",
		"+",
		"+12345678901234567890",
		"invalid-chars!@#$",
		"7900+1234567",
		"\x00\x00\x00",
		"8-800-555-35-35",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		res, err := NormalizePhoneNumber(input, "+7")
		if err != nil {
			// Expected for invalid inputs — verify result is empty
			if res != "" {
				t.Fatalf("expected empty result on error, got %q for input %q", res, input)
			}
			return
		}

		// Invariant checks on valid outputs
		if strings.HasPrefix(res, "+") {
			digitsOnly := res[1:]
			if len(digitsOnly) < 2 || len(digitsOnly) > 15 {
				t.Fatalf("E.164 digits length out of bounds [%d]: %q for input %q", len(digitsOnly), res, input)
			}
		} else {
			// Short code
			if len(res) < 2 || len(res) > 6 {
				t.Fatalf("short code length out of bounds [%d]: %q for input %q", len(res), res, input)
			}
		}
	})
}
