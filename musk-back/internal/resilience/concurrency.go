package resilience

import (
	"context"
	"errors"
	"time"
)

// ErrBusy is returned when the concurrency limiter cannot acquire a slot.
var ErrBusy = errors.New("concurrency limit reached")

// ConcurrencyLimiter caps in-flight work with a semaphore.
type ConcurrencyLimiter struct {
	slots chan struct{}
}

// NewConcurrencyLimiter allows at most max concurrent holders.
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	if max < 1 {
		max = 8
	}
	return &ConcurrencyLimiter{slots: make(chan struct{}, max)}
}

// Acquire blocks until a slot is free or ctx is done.
func (c *ConcurrencyLimiter) Acquire(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case c.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire waits up to timeout for a slot.
func (c *ConcurrencyLimiter) TryAcquire(timeout time.Duration) error {
	if c == nil {
		return nil
	}
	if timeout <= 0 {
		select {
		case c.slots <- struct{}{}:
			return nil
		default:
			return ErrBusy
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case c.slots <- struct{}{}:
		return nil
	case <-timer.C:
		return ErrBusy
	}
}

// Release frees a slot. Safe to call when c is nil.
func (c *ConcurrencyLimiter) Release() {
	if c == nil {
		return
	}
	select {
	case <-c.slots:
	default:
	}
}

// InFlight is the number of currently held slots.
func (c *ConcurrencyLimiter) InFlight() int {
	if c == nil {
		return 0
	}
	return len(c.slots)
}
