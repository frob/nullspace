package jobs

import (
	"context"
	"errors"
	"time"
)

// ErrJobNotFound is returned when a job ID does not exist in the store.
var ErrJobNotFound = errors.New("jobs: not found")

// ErrJobNotPending is returned when an operation requires pending status but the job is not.
var ErrJobNotPending = errors.New("jobs: job is not pending")

// ErrJobNotLeased is returned when ExtendLease is called on a job that is not currently leased.
var ErrJobNotLeased = errors.New("jobs: job is not leased")

// ListFilter constrains which jobs List returns.
type ListFilter struct {
	Statuses []JobStatus // empty means any status
	Queue    string      // empty means any queue
	Type     string      // empty means any type
	Limit    int         // 0 means no cap
}

// Store is the persistence interface for jobs. All implementations must be safe for concurrent use.
type Store interface {
	// Enqueue adds a new job, applying defaults for missing fields.
	Enqueue(ctx context.Context, j *Job) error
	// Lease claims up to max pending jobs from the given queues for the given duration.
	Lease(ctx context.Context, workerID string, queues []string, leaseFor time.Duration, max int) ([]*Job, error)
	// Ack marks a leased job as done.
	Ack(ctx context.Context, jobID string) error
	// Nack returns a leased job to pending with a retry time and error annotation.
	Nack(ctx context.Context, jobID string, retryAt time.Time, lastErr string) error
	// Fail marks a job as permanently failed.
	Fail(ctx context.Context, jobID string, lastErr string) error
	// Cancel cancels a pending job. Returns ErrJobNotPending if not in pending state.
	Cancel(ctx context.Context, jobID string) error
	// Get retrieves a defensive copy of a job by ID.
	Get(ctx context.Context, jobID string) (*Job, error)
	// List returns jobs matching filter.
	List(ctx context.Context, filter ListFilter) ([]*Job, error)
	// ExtendLease updates the LockedUntil timestamp on a currently-leased job.
	// Returns ErrJobNotFound if the job does not exist.
	// Returns ErrJobNotLeased if the job is not in leased status.
	ExtendLease(ctx context.Context, jobID string, until time.Time) error
}
