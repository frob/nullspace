package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/frob/nullspace/core/nslog"
)

// workerPool manages the lifecycle of background job processor goroutines.
type workerPool struct {
	done     chan struct{} // closed to signal workers to stop polling
	wg       sync.WaitGroup
	mu       sync.Mutex
	inflight map[string]bool // jobID -> true while handler is running
	store    Store
}

// startWorkers starts cfg.Workers goroutines that poll for and execute jobs.
func (m *Module) startWorkers(ctx context.Context) *workerPool {
	p := &workerPool{
		done:     make(chan struct{}),
		inflight: make(map[string]bool),
		store:    m.store,
	}

	for i := 0; i < m.cfg.Workers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		p.wg.Add(1)
		go func(id string) {
			defer p.wg.Done()
			m.pollLoop(ctx, p, id)
		}(workerID)
	}

	return p
}

// pollLoop is the main loop for a single worker goroutine.
func (m *Module) pollLoop(ctx context.Context, p *workerPool, workerID string) {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			jobs, err := m.store.Lease(ctx, workerID, []string{"default"}, m.cfg.LeaseDuration, 1)
			if err != nil {
				m.kernel.Logger().Error("jobs: lease error", "worker", workerID, "error", err)
				continue
			}
			for _, j := range jobs {
				p.mu.Lock()
				p.inflight[j.ID] = true
				p.mu.Unlock()

				m.runJob(ctx, p, j)

				p.mu.Lock()
				delete(p.inflight, j.ID)
				p.mu.Unlock()
			}
		}
	}
}

// runJob executes a single job: fires hooks, runs the handler, acks or nacks.
func (m *Module) runJob(ctx context.Context, p *workerPool, job *Job) {
	// Build per-job logger and inject into context.
	logger := m.kernel.Logger().With(
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts+1,
	)
	ctx = nslog.WithLogger(ctx, logger)

	// Inject job identity into context for hooks and handlers.
	ctx = context.WithValue(ctx, JobIDKey, job.ID)
	ctx = context.WithValue(ctx, JobTypeKey, job.Type)

	// Bound context to lease expiry.
	ctx, cancel := context.WithDeadline(ctx, job.LockedUntil)
	defer cancel()

	// Fire before hook.
	_ = m.kernel.Fire("jobs.before", ctx)

	// Look up handler.
	h, err := m.registry.Lookup(job.Type)
	if err != nil {
		// Unknown handler — permanently fail without consuming attempts.
		errMsg := err.Error()
		_ = m.store.Fail(ctx, job.ID, errMsg)
		_ = m.kernel.Fire("jobs.dead", ctx)
		return
	}

	// Run handler with panic recovery.
	var handlerErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				handlerErr = fmt.Errorf("panic: %v", r)
			}
		}()
		handlerErr = h(ctx, job)
	}()

	if handlerErr == nil {
		// Success.
		_ = m.kernel.Fire("jobs.after", ctx)
		_ = m.store.Ack(ctx, job.ID)
		return
	}

	// Failure path.
	errMsg := handlerErr.Error()
	ctx = context.WithValue(ctx, jobErrorKey, handlerErr)
	_ = m.kernel.Fire("jobs.error", ctx)

	attempts := job.Attempts + 1
	if attempts >= job.MaxAttempts {
		_ = m.store.Fail(ctx, job.ID, errMsg)
		_ = m.kernel.Fire("jobs.dead", ctx)
		return
	}

	// Determine retry time: resolve hook first, then exponential backoff.
	retryAt := m.resolveRetryAt(ctx, attempts)
	_ = m.store.Nack(ctx, job.ID, retryAt, errMsg)
}

// resolveRetryAt returns the time at which the job should next be retried.
// It first tries the jobs.retry resolve hook; falls back to exponential backoff.
func (m *Module) resolveRetryAt(ctx context.Context, attempts int) time.Time {
	val, err := m.kernel.Resolve("jobs.retry", ctx)
	if err == nil && val != nil {
		if t, ok := val.(time.Time); ok {
			return t
		}
	}
	return time.Now().Add(m.backoff.Next(attempts))
}

// stop signals all workers to stop polling and waits for them to drain.
// In-flight jobs that do not finish before ctx is cancelled are Nack'd back
// to pending so a subsequent worker can pick them up.
func (p *workerPool) stop(ctx context.Context) {
	// Signal poll loops to exit.
	close(p.done)

	// Wait for workers to finish, but respect the deadline.
	finished := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		// Clean shutdown — nothing to Nack.
		return
	case <-ctx.Done():
		// Deadline exceeded — Nack any jobs that are still in-flight.
		p.mu.Lock()
		inflight := make([]string, 0, len(p.inflight))
		for id := range p.inflight {
			inflight = append(inflight, id)
		}
		p.mu.Unlock()

		for _, id := range inflight {
			_ = p.store.Nack(context.Background(), id, time.Now(), "drained")
		}
	}
}
