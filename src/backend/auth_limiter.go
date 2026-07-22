package main

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	authMaxFailures   = 100
	authFailureWindow = 5 * time.Minute
	authBlockDuration = 5 * time.Minute
)

type authAttempt struct {
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
}

type authAttemptLimiter struct {
	mu            sync.Mutex
	attempts      map[string]authAttempt
	maxFailures   int
	failureWindow time.Duration
	blockDuration time.Duration
}

var globalAuthAttemptLimiter = newAuthAttemptLimiter(
	authMaxFailures,
	authFailureWindow,
	authBlockDuration,
)

func newAuthAttemptLimiter(maxFailures int, failureWindow time.Duration, blockDuration time.Duration) *authAttemptLimiter {
	return &authAttemptLimiter{
		attempts:      make(map[string]authAttempt),
		maxFailures:   maxFailures,
		failureWindow: failureWindow,
		blockDuration: blockDuration,
	}
}

func (l *authAttemptLimiter) retryAfter(client string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[client]
	if !ok || attempt.blockedUntil.IsZero() {
		return 0, false
	}
	retryAfter := attempt.blockedUntil.Sub(now)
	if retryAfter <= 0 {
		delete(l.attempts, client)
		return 0, false
	}
	return retryAfter, true
}

func (l *authAttemptLimiter) failure(client string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt := l.attempts[client]
	if retryAfter := attempt.blockedUntil.Sub(now); retryAfter > 0 {
		return retryAfter, true
	}
	if attempt.lastFailure.IsZero() || now.Sub(attempt.lastFailure) >= l.failureWindow {
		attempt.failures = 0
	}
	attempt.failures++
	attempt.lastFailure = now
	if attempt.failures >= l.maxFailures {
		attempt.blockedUntil = now.Add(l.blockDuration)
	}
	l.attempts[client] = attempt

	if attempt.blockedUntil.IsZero() {
		return 0, false
	}
	return l.blockDuration, true
}

func (l *authAttemptLimiter) success(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, client)
}

func authClient(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func rejectRateLimited(w http.ResponseWriter, r *http.Request, logger *slog.Logger) bool {
	now := time.Now()
	retryAfter, limited := globalAuthAttemptLimiter.retryAfter(authClient(r), now)
	if !limited {
		return false
	}
	writeRateLimitResponse(w, logger, retryAfter, now.Add(retryAfter))
	return true
}

func recordAuthFailure(w http.ResponseWriter, r *http.Request, logger *slog.Logger) bool {
	now := time.Now()
	retryAfter, limited := globalAuthAttemptLimiter.failure(authClient(r), now)
	if !limited {
		return false
	}
	writeRateLimitResponse(w, logger, retryAfter, now.Add(retryAfter))
	return true
}

func writeRateLimitResponse(
	w http.ResponseWriter,
	logger *slog.Logger,
	retryAfter time.Duration,
	resetAt time.Time,
) {
	seconds := max(1, int((retryAfter+time.Second-1)/time.Second))
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	w.Header().Set("RateLimit-Reset", strconv.Itoa(seconds))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(globalAuthAttemptLimiter.maxFailures))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
	w.WriteHeader(http.StatusTooManyRequests)
	writeBody(logger, w, []byte("Too many authentication attempts"))
}
