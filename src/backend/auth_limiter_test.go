package main

import (
	"testing"
	"time"
)

func TestAuthAttemptLimiter(t *testing.T) {
	start := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)

	t.Run("blocks at failure limit and expires", func(t *testing.T) {
		limiter := newAuthAttemptLimiter(2, time.Minute, 5*time.Minute)
		if _, limited := limiter.failure("client", start); limited {
			t.Fatal("first failure unexpectedly rate limited")
		}
		if retryAfter, limited := limiter.failure("client", start.Add(time.Second)); !limited || retryAfter != 5*time.Minute {
			t.Fatalf("second failure limited = %t, retryAfter = %v; want true, 5m", limited, retryAfter)
		}
		if retryAfter, limited := limiter.retryAfter("client", start.Add(2*time.Minute)); !limited || retryAfter != 3*time.Minute+time.Second {
			t.Fatalf("active block limited = %t, retryAfter = %v; want true, 3m1s", limited, retryAfter)
		}
		if _, limited := limiter.retryAfter("client", start.Add(6*time.Minute)); limited {
			t.Fatal("expired block remained rate limited")
		}
	})

	t.Run("success clears failures", func(t *testing.T) {
		limiter := newAuthAttemptLimiter(2, time.Minute, time.Minute)
		limiter.failure("client", start)
		limiter.success("client")
		if _, limited := limiter.failure("client", start.Add(time.Second)); limited {
			t.Fatal("failure after success unexpectedly reached the previous limit")
		}
	})

	t.Run("failure window resets", func(t *testing.T) {
		limiter := newAuthAttemptLimiter(2, time.Minute, time.Minute)
		limiter.failure("client", start)
		if _, limited := limiter.failure("client", start.Add(2*time.Minute)); limited {
			t.Fatal("failure outside the window unexpectedly reached the limit")
		}
	})

	t.Run("clients are isolated", func(t *testing.T) {
		limiter := newAuthAttemptLimiter(1, time.Minute, time.Minute)
		limiter.failure("blocked", start)
		if _, limited := limiter.retryAfter("other", start); limited {
			t.Fatal("one client's failures blocked another client")
		}
	})
}
