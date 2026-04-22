package circuitbreaker

import (
	"errors"
	"log"
	"sync"
	"time"
)

// State represents the circuit breaker's current state.
//
// WHY three states instead of just on/off?
// HALF-OPEN is the key insight. Without it, the breaker
// would stay open forever (never recover) or slam the
// recovering service with full traffic immediately.
// HALF-OPEN lets exactly ONE request through as a probe.
// If it succeeds → close the circuit (service is back).
// If it fails → re-open (still broken, wait longer).
type State int

const (
	StateClosed   State = iota // Normal operation — requests flow through
	StateOpen                  // Tripped — requests rejected immediately
	StateHalfOpen              // Testing — one probe request allowed
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}

// ErrCircuitOpen is returned when the breaker is open.
// Callers check for this error to know they should use a fallback.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Breaker implements the circuit breaker pattern.
//
// WHY these specific thresholds?
// - maxFailures=5: tolerates brief network hiccups (1-2 failures)
//   without tripping. 5 consecutive failures = real outage.
// - timeout=30s: gives the failed service time to restart.
//   Too short = constant half-open probing adds load.
//   Too long = unnecessarily slow recovery.
//
// In production, these would be configurable per-service.
type Breaker struct {
	mu          sync.Mutex
	state       State
	failures    int
	maxFailures int
	timeout     time.Duration
	lastFailure time.Time
	name        string
}

// New creates a circuit breaker with the given name and thresholds.
func New(name string, maxFailures int, timeout time.Duration) *Breaker {
	return &Breaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		timeout:     timeout,
		name:        name,
	}
}

// Execute runs the given function through the circuit breaker.
//
// WHY wrap the function instead of checking state manually?
// This pattern (Execute wraps your call) ensures you can't
// forget to record success/failure. Every call is tracked.
// Manual state checking leads to inconsistent bookkeeping.
func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()

	switch b.state {
	case StateOpen:
		// Check if timeout has elapsed — maybe it's time to probe
		if time.Since(b.lastFailure) > b.timeout {
			b.state = StateHalfOpen
			log.Printf("[CircuitBreaker:%s] State: OPEN → HALF-OPEN (probing)", b.name)
			b.mu.Unlock()
			return b.doExecute(fn)
		}
		b.mu.Unlock()
		return ErrCircuitOpen

	case StateHalfOpen:
		b.mu.Unlock()
		return b.doExecute(fn)

	default: // StateClosed
		b.mu.Unlock()
		return b.doExecute(fn)
	}
}

func (b *Breaker) doExecute(fn func() error) error {
	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.failures++
		b.lastFailure = time.Now()

		if b.state == StateHalfOpen {
			// Probe failed — back to open
			b.state = StateOpen
			log.Printf("[CircuitBreaker:%s] State: HALF-OPEN → OPEN (probe failed)", b.name)
		} else if b.failures >= b.maxFailures {
			// Too many failures — trip the breaker
			b.state = StateOpen
			log.Printf("[CircuitBreaker:%s] State: CLOSED → OPEN (after %d failures)", b.name, b.failures)
		}
		return err
	}

	// Success — reset everything
	if b.state != StateClosed {
		log.Printf("[CircuitBreaker:%s] State: %s → CLOSED (recovered)", b.name, b.state)
	}
	b.failures = 0
	b.state = StateClosed
	return nil
}

// State returns the current breaker state.
func (b *Breaker) GetState() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Failures returns the current failure count.
func (b *Breaker) Failures() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures
}
