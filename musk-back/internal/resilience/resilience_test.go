package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	n := 0
	err := DefaultRetry().Do(context.Background(), func(attempt int) error {
		n = attempt
		if attempt < 3 {
			return errors.New("temporarily unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if n != 3 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestRetryStopsOnNonRetryable(t *testing.T) {
	n := 0
	permanent := errors.New("invalid json")
	err := DefaultRetry().Do(context.Background(), func(attempt int) error {
		n = attempt
		return permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("err = %v", err)
	}
	if n != 1 {
		t.Fatalf("attempts = %d, want 1", n)
	}
}

func TestRetryHonorsProviderRetryIn(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 2, Backoff: time.Hour}
	start := time.Now()
	n := 0
	err := p.Do(context.Background(), func(int) error {
		n++
		if n == 1 {
			return errors.New("status code: 429 Please retry in 0.05s")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if n != 2 {
		t.Fatalf("attempts = %d", n)
	}
	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("elapsed = %s", elapsed)
	}
}

func TestRetrySkipsLongQuotaDelay(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 2, Backoff: 10 * time.Millisecond}
	start := time.Now()
	err := p.Do(context.Background(), func(int) error {
		return errors.New("429 Please retry in 14h4m55s")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("waited for daily quota: %s", time.Since(start))
	}
}

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	b := NewCircuitBreaker(2, 30*time.Millisecond)
	if !b.Allow() {
		t.Fatal("closed should allow")
	}
	b.Fail()
	b.Fail()
	if b.State() != StateOpen {
		t.Fatalf("state = %s, want open", b.State())
	}
	if b.Allow() {
		t.Fatal("open should reject")
	}
	time.Sleep(40 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("half-open should allow")
	}
	b.Succeed()
	if b.State() != StateClosed {
		t.Fatalf("state = %s, want closed", b.State())
	}
}

func TestRateLimiterBurstThenBlocks(t *testing.T) {
	l := NewRateLimiter(1, 2)
	if !l.Allow() || !l.Allow() {
		t.Fatal("burst should allow 2")
	}
	if l.Allow() {
		t.Fatal("third token should be denied")
	}
}

func TestConcurrencyLimiterCapsInFlight(t *testing.T) {
	c := NewConcurrencyLimiter(2)
	if err := c.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.TryAcquire(5 * time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want busy", err)
	}
	c.Release()
	if err := c.TryAcquire(20 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var inFlight atomic.Int32
	var wg sync.WaitGroup
	gate := NewConcurrencyLimiter(3)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := gate.Acquire(context.Background()); err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			cur := inFlight.Add(1)
			if cur > 3 {
				t.Errorf("in flight %d > 3", cur)
			}
			time.Sleep(5 * time.Millisecond)
			inFlight.Add(-1)
			gate.Release()
		}()
	}
	wg.Wait()
}
