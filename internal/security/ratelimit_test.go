package security

import "testing"

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
