package resilience

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// RetryPolicy retries a fallible operation with exponential backoff.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

// DefaultRetry is three attempts with 200ms base backoff.
func DefaultRetry() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, Backoff: 200 * time.Millisecond}
}

func (p RetryPolicy) attempts() int {
	if p.MaxAttempts < 1 {
		return 1
	}
	return p.MaxAttempts
}

func (p RetryPolicy) backoff() time.Duration {
	if p.Backoff <= 0 {
		return 200 * time.Millisecond
	}
	return p.Backoff
}

// Do runs fn until it succeeds, is non-retryable, or attempts are exhausted.
func (p RetryPolicy) Do(ctx context.Context, fn func(attempt int) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if fn == nil {
		return errors.New("retry function is required")
	}
	var last error
	attempts := p.attempts()
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = fn(i + 1)
		if last == nil {
			return nil
		}
		if i == attempts-1 || !Retryable(last) {
			return last
		}
		delay := p.backoff() * time.Duration(1<<uint(i))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

// Retryable reports whether err is transient (timeout, temporary network, 429/5xx-like).
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return true
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"):
		return true
	case strings.Contains(msg, "temporarily"):
		return true
	case strings.Contains(msg, "unavailable"):
		return true
	case strings.Contains(msg, "connection reset"):
		return true
	case strings.Contains(msg, "connection refused"):
		return true
	case strings.Contains(msg, "too many requests"):
		return true
	case strings.Contains(msg, "429"):
		return true
	case strings.Contains(msg, "502"), strings.Contains(msg, "503"), strings.Contains(msg, "504"):
		return true
	default:
		return false
	}
}
