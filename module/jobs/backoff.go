package jobs

import (
	"math"
	"math/rand/v2"
	"time"
)

// BackoffStrategy determines how long to wait before retrying a failed job.
type BackoffStrategy interface {
	Next(attempt int) time.Duration
}

// ExponentialBackoff implements BackoffStrategy with optional jitter.
// The delay doubles each attempt: Base * 2^(attempt-1), capped at Max.
// If Jitter > 0, a uniform random value in [0, delay*Jitter) is added.
type ExponentialBackoff struct {
	Base   time.Duration
	Max    time.Duration
	Jitter float64
}

// Next returns the backoff duration for the given attempt (1-based).
// Attempts ≤ 0 return Base.
func (b ExponentialBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return b.Base
	}

	exp := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(b.Base) * exp)
	if delay > b.Max || delay < 0 { // overflow guard
		delay = b.Max
	}

	if b.Jitter > 0 {
		jitterRange := float64(delay) * b.Jitter
		delay += time.Duration(rand.Float64() * jitterRange)
	}

	return delay
}
