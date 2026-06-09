// Package jobs provides background job scheduling and execution primitives.
package jobs

import (
	"encoding/json"
	"time"
)

// JobStatus is the lifecycle state of a job.
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusLeased    JobStatus = "leased"
	StatusDone      JobStatus = "done"
	StatusFailed    JobStatus = "failed"
	StatusCancelled JobStatus = "cancelled"
)

// Job represents a single unit of deferred work.
type Job struct {
	ID          string
	Type        string
	Queue       string
	Payload     []byte
	Status      JobStatus
	Attempts    int
	MaxAttempts int
	RunAt       time.Time
	LockedUntil time.Time
	LockedBy    string
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// JobSpec carries the parameters used to enqueue a new job.
type JobSpec struct {
	Type        string
	Payload     any
	RunAt       time.Time
	MaxAttempts int
	Queue       string
}

// EncodePayload serialises v to JSON bytes suitable for Job.Payload.
// nil returns (nil, nil); a []byte is returned as-is without re-encoding.
func EncodePayload(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	return json.Marshal(v)
}

// DecodePayload deserialises data into into using JSON.
func DecodePayload(data []byte, into any) error {
	return json.Unmarshal(data, into)
}
