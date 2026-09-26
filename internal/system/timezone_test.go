package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveLocation_FixedOffset(t *testing.T) {
	loc, err := ResolveLocation("+03:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC).In(loc)
	_, offset := now.Zone()
	if offset != 3*3600 {
		t.Errorf("expected offset %d, got %d", 3*3600, offset)
	}

	loc2, err := ResolveLocation("-05:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, offset2 := now.In(loc2).Zone()
	if offset2 != -5*3600 {
		t.Errorf("expected offset %d, got %d", -5*3600, offset2)
	}
}

func TestResolveLocation_POSIX(t *testing.T) {
	loc, err := ResolveLocation("MSK-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC).In(loc)
	_, offset := now.Zone()
	if offset != 3*3600 {
		t.Errorf("expected offset %d for MSK-3, got %d", 3*3600, offset)
	}

	loc2, err := ResolveLocation("<+07>-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, offset2 := now.In(loc2).Zone()
	if offset2 != 7*3600 {
		t.Errorf("expected offset %d for <+07>-7, got %d", 7*3600, offset2)
	}
}

func TestResolveLocation_FromEnvTZ(t *testing.T) {
	orig := os.Getenv("TZ")
	defer os.Setenv("TZ", orig)

	os.Setenv("TZ", "+04:00")
	loc, err := ResolveLocation("auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC).In(loc)
	_, offset := now.Zone()
	if offset != 4*3600 {
		t.Errorf("expected offset %d, got %d", 4*3600, offset)
	}
}

func TestFormatLocalTime(t *testing.T) {
	loc := time.FixedZone("TEST+03", 3*3600)
	tm := time.Date(2026, 9, 26, 12, 34, 56, 789000000, time.UTC)
	formatted := FormatLocalTime(tm, loc)
	expectedPrefix := "2026-09-26T15:34:56.789+03:00"
	if formatted != expectedPrefix {
		t.Errorf("expected %s, got %s", expectedPrefix, formatted)
	}
}

func TestResolveLocation_FileFallback(t *testing.T) {
	tmpDir := t.TempDir()
	tzFile := filepath.Join(tmpDir, "TZ")
	if err := os.WriteFile(tzFile, []byte("MSK-3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("TZ")
	defer os.Setenv("TZ", orig)
	os.Unsetenv("TZ")

	// Test parseLocation directly on the file content
	data, _ := os.ReadFile(tzFile)
	loc, err := parseLocation(string(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC).In(loc)
	_, offset := now.Zone()
	if offset != 3*3600 {
		t.Errorf("expected offset %d, got %d", 3*3600, offset)
	}
}
