package pdu

import (
	"testing"
)

// FuzzDecodeSMS uses Go coverage-guided fuzzing to verify that DecodeSMS never crashes or panics
// on any arbitrary, malformed, or corrupt hex string.
func FuzzDecodeSMS(f *testing.F) {
	seeds := []string{
		"00040B919712345678F900006290229000002305C8329BFD0E",
		"00040B919712345678F90008629022900000230C041F04400438043204350442",
		"00440B919712345678F90008629022900000230E050003A702010422043504410442",
		"",
		"00",
		"00040",
		"00ZZZZZZZZ",
		"05910000",
		"FFFFFFFFFFFFFFFFFFFF",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, hexInput string) {
		// Invariant: DecodeSMS must never panic on any input
		_, _ = DecodeSMS(hexInput)
	})
}
