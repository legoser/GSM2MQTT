package main

import (
	"strings"
	"testing"
)

// TestIsVersionFlag verifies --version / -v detection (positive + negative cases).
func TestIsVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "long flag only", args: []string{"--version"}, want: true},
		{name: "short flag only", args: []string{"-v"}, want: true},
		{name: "flag among other args", args: []string{"--config", "x.yaml", "--version"}, want: true},
		{name: "short flag among other args", args: []string{"-v", "--config", "x.yaml"}, want: true},
		{name: "no args", args: nil, want: false},
		{name: "empty args", args: []string{}, want: false},
		{name: "config flag only", args: []string{"--config", "x.yaml"}, want: false},
		{name: "case sensitive uppercase", args: []string{"--Version"}, want: false},
		{name: "capital short flag", args: []string{"-V"}, want: false},
		{name: "version as value", args: []string{"--config=version"}, want: false},
		{name: "prefix match rejected", args: []string{"--versioned"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVersionFlag(tt.args); got != tt.want {
				t.Errorf("isVersionFlag(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestVersionFormat verifies Version() embeds name and build marker.
func TestVersionFormat(t *testing.T) {
	v := Version()
	if !strings.HasPrefix(v, "gsm2mqtt ") {
		t.Errorf("Version() = %q, want prefix %q", v, "gsm2mqtt ")
	}
	if !strings.Contains(v, "(built ") || !strings.HasSuffix(v, ")") {
		t.Errorf("Version() = %q, want %q marker with closing paren", v, "(built ...)")
	}
	if strings.Contains(v, "\n") {
		t.Errorf("Version() = %q, must be single line", v)
	}
}
