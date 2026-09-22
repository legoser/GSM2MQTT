package security

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	limiter := NewRateLimiter(3)

	for i := 0; i < 3; i++ {
		if !limiter.Allow("+79991112233") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}

	// 4th attempt should be blocked
	if limiter.Allow("+79991112233") {
		t.Errorf("4th attempt should be blocked due to rate limit")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled: false,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	// Should allow many requests without limit
	for i := 0; i < 50; i++ {
		if err := limiter.Check("+79991112233"); err != nil {
			t.Fatalf("disabled rate limiter should allow all attempts, got err at %d: %v", i, err)
		}
	}
}

func TestRateLimiter_MinuteLimit(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled:      true,
		MaxPerMinute: 2,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	if err := limiter.Check("+79991112233"); err != nil {
		t.Fatalf("attempt 1 error: %v", err)
	}
	if err := limiter.Check("+79991112233"); err != nil {
		t.Fatalf("attempt 2 error: %v", err)
	}

	// Attempt 3 exceeds minute limit
	err := limiter.Check("+79991112233")
	if !errors.Is(err, ErrRateLimitMinuteExceeded) {
		t.Fatalf("expected ErrRateLimitMinuteExceeded, got: %v", err)
	}
}

func TestRateLimiter_HourLimit(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled:      true,
		MaxPerMinute: 10,
		MaxPerHour:   3,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	for i := 0; i < 3; i++ {
		if err := limiter.Check("+79991112233"); err != nil {
			t.Fatalf("attempt %d error: %v", i+1, err)
		}
	}

	// 4th attempt exceeds hour limit
	err := limiter.Check("+79991112233")
	if !errors.Is(err, ErrRateLimitHourExceeded) {
		t.Fatalf("expected ErrRateLimitHourExceeded, got: %v", err)
	}
}

func TestRateLimiter_DayLimit(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled:      true,
		MaxPerMinute: 10,
		MaxPerHour:   10,
		MaxPerDay:    4,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	for i := 0; i < 4; i++ {
		if err := limiter.Check("+79991112233"); err != nil {
			t.Fatalf("attempt %d error: %v", i+1, err)
		}
	}

	// 5th attempt exceeds day limit
	err := limiter.Check("+79991112233")
	if !errors.Is(err, ErrRateLimitDayExceeded) {
		t.Fatalf("expected ErrRateLimitDayExceeded, got: %v", err)
	}
}

func TestRateLimiter_PerNumberLimit(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled:             true,
		MaxPerMinute:        10,
		MaxPerHour:          10,
		MaxPerDay:           10,
		MaxPerNumberPerHour: 2,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	// Send 2 to number A
	if err := limiter.Check("+79991112233"); err != nil {
		t.Fatalf("number A attempt 1 error: %v", err)
	}
	if err := limiter.Check("+79991112233"); err != nil {
		t.Fatalf("number A attempt 2 error: %v", err)
	}

	// 3rd to number A should fail
	errA := limiter.Check("+79991112233")
	if !errors.Is(errA, ErrRateLimitPerNumberExceeded) {
		t.Fatalf("expected ErrRateLimitPerNumberExceeded for number A, got: %v", errA)
	}

	// Send to number B should still succeed
	if err := limiter.Check("+79998887766"); err != nil {
		t.Fatalf("number B attempt 1 should succeed, got: %v", err)
	}
}

func TestRateLimiter_Cooldown(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled:      true,
		MaxPerMinute: 2,
		Cooldown:     100 * time.Millisecond,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	_ = limiter.Check("+79991112233")
	_ = limiter.Check("+79991112233")

	// Trigger cooldown
	err := limiter.Check("+79991112233")
	if !errors.Is(err, ErrRateLimitMinuteExceeded) {
		t.Fatalf("expected minute limit error, got: %v", err)
	}

	// Immediate next check should return ErrRateLimitCooldown
	errCooldown := limiter.Check("+79991112233")
	if !errors.Is(errCooldown, ErrRateLimitCooldown) {
		t.Fatalf("expected ErrRateLimitCooldown, got: %v", errCooldown)
	}

	// Wait for cooldown to expire
	time.Sleep(120 * time.Millisecond)

	// Limit should still apply or allow depending on minute window, but cooldown is gone
	limiter.Reset()
	if err := limiter.Check("+79991112233"); err != nil {
		t.Fatalf("expected check to succeed after Reset, got: %v", err)
	}
}

func TestRateLimiter_EmptyNumber(t *testing.T) {
	limiter := NewRateLimiter(5)
	err := limiter.Check("")
	if !errors.Is(err, ErrEmptyPhoneNumber) {
		t.Errorf("expected ErrEmptyPhoneNumber for empty number, got: %v", err)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	cfg := RateLimiterConfig{
		Enabled:      true,
		MaxPerMinute: 1000,
	}
	limiter := NewRateLimiterWithConfig(cfg)

	var wg sync.WaitGroup
	workers := 20
	requestsPerWorker := 30

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				_ = limiter.Allow("+79991112233")
			}
		}()
	}

	wg.Wait()
}
