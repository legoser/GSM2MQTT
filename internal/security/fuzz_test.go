package security

import (
	"strings"
	"testing"
)

func FuzzFilter(f *testing.F) {
	seeds := []string{
		"+79991112233",
		"89991112233",
		"112",
		"900",
		"+1 (555) 123-4567",
		"Google",
		"MCHS",
		"",
		" ",
		"+7\x00999",
		"AT+CFUN=0",
		"some-alphanumeric-name",
		"+79998887766\r\n",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	filterWhitelist := NewFilter("whitelist", []string{"+79991112233", "Google"}, nil)
	filterBlacklist := NewFilter("blacklist", nil, []string{"+79998887766"})
	filterAll := NewFilter("all", nil, nil)

	f.Fuzz(func(t *testing.T, input string) {
		// None of these should panic
		_ = filterWhitelist.Allowed(input)
		_ = filterWhitelist.Check(input)

		_ = filterBlacklist.Allowed(input)
		_ = filterBlacklist.Check(input)

		_ = filterAll.Allowed(input)
		_ = filterAll.Check(input)

		// If input contains null byte or CRLF, Check must return an error
		if strings.ContainsAny(input, "\x00\r\n") {
			if err := filterAll.Check(input); err == nil {
				t.Errorf("expected error on dangerous characters in Check(%q)", input)
			}
		}
	})
}

func FuzzSanitizer(f *testing.F) {
	seeds := []string{
		"ATI",
		"AT+CSQ",
		"AT+COPS?",
		"AT+CFUN=0",
		"AT+CFUN = 0",
		"at+cpin=\"1234\"",
		"ATD+79991112233;",
		"AT&F",
		"",
		"   ",
		"ATI\x00AT+CFUN=0",
		"ATI\x1A",
		"ATI\x1B",
		"ATI\x07",
		"ATI\x0B",
		"ATI\x0C",
		"ATI\x7F",
		"AT+CGDCONT=1,\"IP\",\"internet\"",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	s := NewSanitizer(true, []string{"ATI", "AT+CSQ", "AT+COPS"})

	f.Fuzz(func(t *testing.T, cmd string) {
		// Should never panic
		_ = s.IsAllowed(cmd)
		err := s.Validate(cmd)

		// Invariant: dangerous characters must always be rejected
		if strings.ContainsAny(cmd, "\x00\x1A\x1B\x07\x0B\x0C\x7F\r\n") {
			if err == nil {
				t.Errorf("expected error for command with dangerous chars: %q", cmd)
			}
		}

		// Invariant: prefix trap — "AT+CFUN"/"AT+CPIN"/"AT+CMGS" must never
		// be authorized by the narrow allowlist above.
		if err == nil {
			upper := canonicalizeCommand(cmd)
			for _, evil := range []string{"AT+CFUN", "AT+CPIN", "AT+CMGS", "ATD", "AT&F"} {
				if len(upper) >= len(evil) && upper[:len(evil)] == evil {
					t.Errorf("allowlist bypass for %q", cmd)
				}
			}
		}
	})
}
