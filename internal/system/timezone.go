// Package system provides system environment and timezone helpers for GSM2MQTT.
package system

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var fixedOffsetRegex = regexp.MustCompile(`^([+-])(\d{1,2})(?::?(\d{2}))?$`)
var posixBracketRegex = regexp.MustCompile(`^<([A-Za-z0-9_+-]+)>([+-]?\d+)(?::(\d+))?`)
var posixSimpleRegex = regexp.MustCompile(`^([A-Za-z]+)([+-]?\d+)(?::(\d+))?`)

// ResolveLocation determines the time.Location based on configured string,
// $TZ environment variable, OpenWrt /etc/TZ, or /etc/timezone.
func ResolveLocation(cfgTZ string) (*time.Location, error) {
	tz := strings.TrimSpace(cfgTZ)
	if tz != "" && !strings.EqualFold(tz, "auto") && !strings.EqualFold(tz, "local") {
		return parseLocation(tz)
	}

	// 1. Check environment variable TZ
	if envTZ := strings.TrimSpace(os.Getenv("TZ")); envTZ != "" {
		if loc, err := parseLocation(envTZ); err == nil {
			return loc, nil
		}
	}

	// 2. Check OpenWrt / BusyBox /etc/TZ
	if data, err := os.ReadFile("/etc/TZ"); err == nil {
		if val := strings.TrimSpace(string(data)); val != "" {
			if loc, err := parseLocation(val); err == nil {
				return loc, nil
			}
		}
	}

	// 3. Check Debian/Ubuntu /etc/timezone
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		if val := strings.TrimSpace(string(data)); val != "" {
			if loc, err := parseLocation(val); err == nil {
				return loc, nil
			}
		}
	}

	// 4. Fallback to system local
	return time.Local, nil
}

func parseLocation(tz string) (*time.Location, error) {
	// Support fixed numeric offsets like "+03:00", "+07", "-05:00"
	if matches := fixedOffsetRegex.FindStringSubmatch(tz); matches != nil {
		sign := matches[1]
		hours, _ := strconv.Atoi(matches[2])
		mins := 0
		if matches[3] != "" {
			mins, _ = strconv.Atoi(matches[3])
		}
		totalSeconds := hours*3600 + mins*60
		if sign == "-" {
			totalSeconds = -totalSeconds
		}
		name := fmt.Sprintf("UTC%s%02d:%02d", sign, hours, mins)
		return time.FixedZone(name, totalSeconds), nil
	}

	// Try IANA name or standard location (e.g. "Europe/Moscow", "UTC")
	loc, err := time.LoadLocation(tz)
	if err == nil {
		return loc, nil
	}

	// Support POSIX TZ strings like "MSK-3", "GMT-7", "<+07>-7" commonly found in /etc/TZ on OpenWrt
	// POSIX standard: name followed by offset from UTC where sign is inverted (e.g. MSK-3 means UTC+3)
	var name string
	var offsetStr, minsStr string

	if pm := posixBracketRegex.FindStringSubmatch(tz); pm != nil {
		name = pm[1]
		offsetStr = pm[2]
		minsStr = pm[3]
	} else if pm := posixSimpleRegex.FindStringSubmatch(tz); pm != nil {
		name = pm[1]
		offsetStr = pm[2]
		minsStr = pm[3]
	}

	if name != "" && offsetStr != "" {
		posixOffset, _ := strconv.Atoi(offsetStr)
		posixMins := 0
		if minsStr != "" {
			posixMins, _ = strconv.Atoi(minsStr)
		}
		// In POSIX TZ, negative offset means EAST of UTC (positive offset in ISO)
		totalSeconds := -(posixOffset*3600 + posixMins*60)
		return time.FixedZone(name, totalSeconds), nil
	}

	return nil, fmt.Errorf("unknown timezone format %q: %w", tz, err)
}

// FormatLocalTime formats a time in the given location with ISO 8601 / RFC 3339 offset.
func FormatLocalTime(t time.Time, loc *time.Location) string {
	if loc != nil {
		t = t.In(loc)
	}
	return t.Format("2006-01-02T15:04:05.000-07:00")
}
