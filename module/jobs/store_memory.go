package jobs

import (
	"container/heap"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MemoryStore is an in-process Store backed by a map and a min-heap on RunAt.
// It is safe for concurrent use.
type MemoryStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
	h    jobHeap
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: make(map[string]*Job),
	}
}

// Enqueue adds j to the store, filling in defaults for missing fields.
func (m *MemoryStore) Enqueue(_ context.Context, j *Job) error {
	now := time.Now()
	if j.ID == "" {
		id, err := generateJobID()
		if err != nil {
			return fmt.Errorf("jobs: generate ID: %w", err)
		}
		j.ID = id
	}
	if j.Status == "" {
		j.Status = StatusPending
	}
	if j.Queue == "" {
		j.Queue = "default"
	}
	if j.RunAt.IsZero() {
		j.RunAt = now
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	if j.UpdatedAt.IsZero() {
		j.UpdatedAt = now
	}

	stored := copyJob(j)
	m.mu.Lock()
	m.jobs[stored.ID] = stored
	heap.Push(&m.h, stored)
	m.mu.Unlock()
	return nil
}

// Lease claims up to max pending jobs from queues, returning defensive copies.
// Expired leases are reclaimed transparently during the scan.
func (m *MemoryStore) Lease(_ context.Context, workerID string, queues []string, leaseFor time.Duration, max int) ([]*Job, error) {
	now := time.Now()
	qset := make(map[string]struct{}, len(queues))
	for _, q := range queues {
		qset[q] = struct{}{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect candidates from the heap in RunAt order using a temp slice,
	// then restore non-selected items back.
	var candidates []*Job
	var skipped []*Job

	for m.h.Len() > 0 {
		top := m.h[0]
		// Refresh from map (status may have changed without heap removal).
		stored, ok := m.jobs[top.ID]
		if !ok {
			heap.Pop(&m.h)
			continue
		}
		// Lazy-sync heap item pointer.
		m.h[0] = stored

		// Nothing past this RunAt will be ready either.
		if stored.RunAt.After(now) {
			break
		}

		heap.Pop(&m.h)

		// Reclaim expired leases.
		if stored.Status == StatusLeased && stored.LockedUntil.After(now) {
			skipped = append(skipped, stored)
			continue
		}

		// Skip jobs not in the requested queues.
		if _, ok := qset[stored.Queue]; !ok {
			skipped = append(skipped, stored)
			continue
		}

		// Skip non-leasable statuses.
		if stored.Status != StatusPending &&
			!(stored.Status == StatusLeased && !stored.LockedUntil.After(now)) {
			skipped = append(skipped, stored)
			continue
		}

		candidates = append(candidates, stored)
		if len(candidates) == max {
			break
		}
	}

	// Push skipped items back.
	for _, j := range skipped {
		heap.Push(&m.h, j)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	result := make([]*Job, len(candidates))
	for i, stored := range candidates {
		stored.Status = StatusLeased
		stored.LockedBy = workerID
		stored.LockedUntil = now.Add(leaseFor)
		stored.UpdatedAt = now
		result[i] = copyJob(stored)
		// Push updated job back onto heap so future Lease calls see it.
		heap.Push(&m.h, stored)
	}
	return result, nil
}

// Ack marks job jobID as done and increments Attempts.
func (m *MemoryStore) Ack(_ context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	j.Status = StatusDone
	j.Attempts++
	j.UpdatedAt = time.Now()
	return nil
}

// Nack returns job jobID to pending with a new RunAt and increments Attempts.
func (m *MemoryStore) Nack(_ context.Context, jobID string, retryAt time.Time, lastErr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	j.Status = StatusPending
	j.Attempts++
	j.LastError = lastErr
	j.RunAt = retryAt
	j.LockedBy = ""
	j.LockedUntil = time.Time{}
	j.UpdatedAt = time.Now()
	// Push back onto heap with updated RunAt; lazy dedup during Lease is fine.
	heap.Push(&m.h, j)
	return nil
}

// Fail marks job jobID as permanently failed.
func (m *MemoryStore) Fail(_ context.Context, jobID string, lastErr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	j.Status = StatusFailed
	j.LastError = lastErr
	j.UpdatedAt = time.Now()
	return nil
}

// Cancel cancels a pending job. Returns ErrJobNotPending if not pending.
func (m *MemoryStore) Cancel(_ context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	if j.Status != StatusPending {
		return ErrJobNotPending
	}
	j.Status = StatusCancelled
	j.UpdatedAt = time.Now()
	return nil
}

// Get returns a defensive copy of job jobID.
func (m *MemoryStore) Get(_ context.Context, jobID string) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return nil, ErrJobNotFound
	}
	c := copyJob(j)
	return c, nil
}

// List returns defensive copies of jobs matching filter.
func (m *MemoryStore) List(_ context.Context, filter ListFilter) ([]*Job, error) {
	statusSet := make(map[JobStatus]struct{}, len(filter.Statuses))
	for _, s := range filter.Statuses {
		statusSet[s] = struct{}{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var out []*Job
	for _, j := range m.jobs {
		if len(statusSet) > 0 {
			if _, ok := statusSet[j.Status]; !ok {
				continue
			}
		}
		if filter.Queue != "" && j.Queue != filter.Queue {
			continue
		}
		if filter.Type != "" && j.Type != filter.Type {
			continue
		}
		out = append(out, copyJob(j))
		if filter.Limit > 0 && len(out) == filter.Limit {
			break
		}
	}
	return out, nil
}

// ExtendLease updates the LockedUntil timestamp on a currently-leased job.
func (m *MemoryStore) ExtendLease(_ context.Context, jobID string, until time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}
	if j.Status != StatusLeased {
		return ErrJobNotLeased
	}
	j.LockedUntil = until
	j.UpdatedAt = time.Now()
	return nil
}

// copyJob returns a shallow struct copy with an independent Payload slice.
func copyJob(j *Job) *Job {
	c := *j
	if j.Payload != nil {
		c.Payload = make([]byte, len(j.Payload))
		copy(c.Payload, j.Payload)
	}
	return &c
}

// generateJobID returns a 16-byte random hex string.
func generateJobID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// jobHeap is a min-heap of *Job ordered by RunAt.
type jobHeap []*Job

func (h jobHeap) Len() int           { return len(h) }
func (h jobHeap) Less(i, j int) bool { return h[i].RunAt.Before(h[j].RunAt) }
func (h jobHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *jobHeap) Push(x any)        { *h = append(*h, x.(*Job)) }
func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}
