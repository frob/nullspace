package jobs

import (
	"testing"
	"time"
)

func TestExponentialBackoff_NoJitter(t *testing.T) {
	t.Parallel()

	b := ExponentialBackoff{
		Base:   1 * time.Second,
		Max:    1 * time.Hour,
		Jitter: 0,
	}

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
	}

	for _, tc := range cases {
		got := b.Next(tc.attempt)
		if got != tc.want {
			t.Errorf("Next(%d): got %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

func TestExponentialBackoff_CappedAtMax(t *testing.T) {
	t.Parallel()

	b := ExponentialBackoff{
		Base:   1 * time.Second,
		Max:    5 * time.Second,
		Jitter: 0,
	}

	// attempt 10 would naively be 2^9 = 512s; must cap at Max=5s.
	got := b.Next(10)
	if got != 5*time.Second {
		t.Fatalf("Next(10): expected exactly Max=5s, got %s", got)
	}
}

func TestExponentialBackoff_JitterBounds(t *testing.T) {
	t.Parallel()

	b := ExponentialBackoff{
		Base:   1 * time.Second,
		Max:    1 * time.Hour,
		Jitter: 0.5,
	}

	// For attempt 3, base delay = 4s; jitter band is [0, 2s); so result is in [4s, 6s).
	const attempt = 3
	const base = 4 * time.Second
	const upper = 6 * time.Second // exclusive

	for i := 0; i < 100; i++ {
		got := b.Next(attempt)
		if got < base {
			t.Fatalf("iter %d: Next(%d)=%s below lower bound %s", i, attempt, got, base)
		}
		if got >= upper {
			t.Fatalf("iter %d: Next(%d)=%s at or above upper bound %s", i, attempt, got, upper)
		}
	}
}

func TestExponentialBackoff_AttemptZeroOrNegative(t *testing.T) {
	t.Parallel()

	// Defensive behavior: attempt <= 0 should return Base (the smallest sensible delay).
	// This documents the chosen contract; if the implementation chooses 0 instead,
	// update this test alongside the implementation.
	b := ExponentialBackoff{
		Base:   1 * time.Second,
		Max:    1 * time.Hour,
		Jitter: 0,
	}

	for _, attempt := range []int{0, -1, -100} {
		got := b.Next(attempt)
		if got != b.Base {
			t.Errorf("Next(%d): expected Base=%s, got %s", attempt, b.Base, got)
		}
	}
}
