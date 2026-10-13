package resilience

import (
	"sync"
	"time"
)

// RateLimiter is a token-bucket limiter.
type RateLimiter struct {
	rps   float64
	burst float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// NewRateLimiter allows rps tokens per second with a burst capacity.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 20
	}
	if burst < 1 {
		burst = int(rps)
		if burst < 1 {
			burst = 1
		}
	}
	now := time.Now()
	return &RateLimiter{
		rps:    rps,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now,
	}
}

// Allow consumes one token. False means the caller should back off.
func (l *RateLimiter) Allow() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.rps
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
