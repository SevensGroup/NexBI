package resilience

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned when the circuit breaker is open and rejecting calls.
var ErrOpen = errors.New("circuit breaker is open")

// State is the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// CircuitBreaker trips open after consecutive failures and recovers after ResetTimeout.
type CircuitBreaker struct {
	failureThreshold int
	resetTimeout     time.Duration

	mu          sync.Mutex
	state       State
	failures    int
	successes   int
	openedAt    time.Time
	halfOpenMax int
}

// NewCircuitBreaker constructs a breaker. threshold < 1 disables tripping (always closed).
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	if failureThreshold < 1 {
		failureThreshold = 5
	}
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		halfOpenMax:      1,
	}
}

// Allow reports whether a call may proceed. False means the breaker is open.
func (b *CircuitBreaker) Allow() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionLocked(time.Now())
	switch b.state {
	case StateOpen:
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

// Succeed records a successful call.
func (b *CircuitBreaker) Succeed() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	if b.state == StateHalfOpen {
		b.successes++
		if b.successes >= b.halfOpenMax {
			b.state = StateClosed
			b.successes = 0
		}
	}
}

// Fail records a failed call and may trip the breaker open.
func (b *CircuitBreaker) Fail() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.transitionLocked(now)
	if b.state == StateHalfOpen {
		b.tripLocked(now)
		return
	}
	b.failures++
	if b.failures >= b.failureThreshold {
		b.tripLocked(now)
	}
}

// State returns the current breaker state.
func (b *CircuitBreaker) State() State {
	if b == nil {
		return StateClosed
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionLocked(time.Now())
	return b.state
}

func (b *CircuitBreaker) tripLocked(now time.Time) {
	b.state = StateOpen
	b.openedAt = now
	b.failures = 0
	b.successes = 0
}

func (b *CircuitBreaker) transitionLocked(now time.Time) {
	if b.state == StateOpen && now.Sub(b.openedAt) >= b.resetTimeout {
		b.state = StateHalfOpen
		b.successes = 0
		b.failures = 0
	}
}
