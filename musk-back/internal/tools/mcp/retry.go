package mcp

import (
	"context"
	"time"
)

func withRetry(ctx context.Context, policy RetryPolicy, fn func(attempt int) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	attempts := policy.attempts()
	var last error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = fn(i + 1)
		if last == nil {
			return nil
		}
		if i == attempts-1 || !retryable(last) {
			return last
		}
		delay := policy.backoff() * time.Duration(1<<uint(i))
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
