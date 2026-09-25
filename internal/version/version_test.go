package version

import (
	"strings"
	"testing"
)

func TestVersion_String(t *testing.T) {
	s := String()
	if !strings.HasPrefix(s, "gsm2mqtt ") {
		t.Errorf("expected string to start with 'gsm2mqtt ', got %q", s)
	}
	if !strings.Contains(s, Version) {
		t.Errorf("expected string to contain Version %q, got %q", Version, s)
	}
}
