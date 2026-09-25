package security

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// RateLimiterConfig configures multi-tier SMS rate limiting.
type RateLimiterConfig struct {
	Enabled             bool
	MaxPerMinute        int
	MaxPerHour          int
	MaxPerDay           int
	MaxPerNumberPerHour int
	Cooldown            time.Duration
}

// RateLimiter limits the frequency of outgoing SMS messages to prevent flooding.
type RateLimiter struct {
	mu               sync.Mutex
	cfg              RateLimiterConfig
	minuteHistory    []time.Time
	hourHistory      []time.Time
	dayHistory       []time.Time
	perNumberHistory map[string][]time.Time
	cooldownUntil    time.Time
}

// NewRateLimiter creates a basic SMS RateLimiter with only a per-minute limit.
func NewRateLimiter(maxPerMinute int) *RateLimiter {
	return NewRateLimiterWithConfig(RateLimiterConfig{
		Enabled:      true,
		MaxPerMinute: maxPerMinute,
	})
}

// NewRateLimiterWithConfig creates a full-featured SMS RateLimiter.
func NewRateLimiterWithConfig(cfg RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		cfg:              cfg,
		perNumberHistory: make(map[string][]time.Time),
	}
}

// Allow returns true if sending an SMS to the number is permitted right now.
func (r *RateLimiter) Allow(number string) bool {
	return r.Check(number) == nil
}

// Check verifies whether an SMS can be sent to the given recipient.
// It returns a typed sentinel error if rate limits are exceeded.
func (r *RateLimiter) Check(number string) error {
	trimmed := strings.TrimSpace(number)
	if trimmed == "" {
		return ErrEmptyPhoneNumber
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.cfg.Enabled {
		return nil
	}

	now := time.Now()
	if now.Before(r.cooldownUntil) {
		slog.Warn("sms rate limiter in cooldown", slog.String("number", MaskPhone(trimmed)), slog.Time("cooldown_until", r.cooldownUntil))
		return ErrRateLimitCooldown
	}

	cleaned := normalizeNumber(trimmed)
	if err := r.checkLimits(now, cleaned); err != nil {
		if r.cfg.Cooldown > 0 {
			r.cooldownUntil = now.Add(r.cfg.Cooldown)
		}
		slog.Warn("sms rate limit exceeded", slog.String("number", MaskPhone(cleaned)), slog.Any("error", err), slog.Duration("cooldown", r.cfg.Cooldown))
		return err
	}

	slog.Debug("sms rate limit passed", slog.String("number", MaskPhone(cleaned)))
	r.recordSend(now, cleaned)
	return nil
}

// Reset clears all counters and cooldown state.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.minuteHistory = nil
	r.hourHistory = nil
	r.dayHistory = nil
	r.perNumberHistory = make(map[string][]time.Time)
	r.cooldownUntil = time.Time{}
}

// checkLimits tests all sliding windows against configured maximums.
func (r *RateLimiter) checkLimits(now time.Time, number string) error {
	r.minuteHistory = pruneOlderThan(r.minuteHistory, now.Add(-time.Minute))
	if r.cfg.MaxPerMinute > 0 && len(r.minuteHistory) >= r.cfg.MaxPerMinute {
		return ErrRateLimitMinuteExceeded
	}

	r.hourHistory = pruneOlderThan(r.hourHistory, now.Add(-time.Hour))
	if r.cfg.MaxPerHour > 0 && len(r.hourHistory) >= r.cfg.MaxPerHour {
		return ErrRateLimitHourExceeded
	}

	r.dayHistory = pruneOlderThan(r.dayHistory, now.Add(-24*time.Hour))
	if r.cfg.MaxPerDay > 0 && len(r.dayHistory) >= r.cfg.MaxPerDay {
		return ErrRateLimitDayExceeded
	}

	numHistory := pruneOlderThan(r.perNumberHistory[number], now.Add(-time.Hour))
	if len(numHistory) == 0 {
		delete(r.perNumberHistory, number)
	} else {
		r.perNumberHistory[number] = numHistory
	}
	if r.cfg.MaxPerNumberPerHour > 0 && len(numHistory) >= r.cfg.MaxPerNumberPerHour {
		return ErrRateLimitPerNumberExceeded
	}

	return nil
}

// recordSend appends current timestamp to all tracking windows.
func (r *RateLimiter) recordSend(now time.Time, number string) {
	r.minuteHistory = append(r.minuteHistory, now)
	r.hourHistory = append(r.hourHistory, now)
	r.dayHistory = append(r.dayHistory, now)
	r.perNumberHistory[number] = append(r.perNumberHistory[number], now)
}

// pruneOlderThan removes timestamps before cutoff.
func pruneOlderThan(timestamps []time.Time, cutoff time.Time) []time.Time {
	idx := 0
	for idx < len(timestamps) && timestamps[idx].Before(cutoff) {
		idx++
	}
	if idx == 0 {
		return timestamps
	}
	return timestamps[idx:]
}
