package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
)

// setupKernel boots a kernel with logging + routing + jobs configured from
// the given TOML body. Mirrors the pattern in module/session/module_test.go.
func setupKernel(t *testing.T, tomlContent string) (*Module, *kernel.Kernel) {
	t.Helper()
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")
	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	logMod := nslog.New()
	routingMod := routing.New()
	jobsMod := New()

	k.Use(logMod)
	k.Use(routingMod)
	k.Use(jobsMod)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return jobsMod, k
}

// — Lifecycle —

func TestModule_Lifecycle(t *testing.T) {
	t.Parallel()
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
store = "memory"
`)
	_ = m

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := k.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestModule_ProvidesResources(t *testing.T) {
	t.Parallel()
	_, k := setupKernel(t, `
[modules]
jobs = true
`)

	gotMod, err := kernel.GetResource[*Module](k, "jobs")
	if err != nil {
		t.Fatalf("GetResource(jobs): %v", err)
	}
	if gotMod == nil {
		t.Fatal("expected non-nil module from resource locator")
	}

	gotReg, err := kernel.GetResource[*HandlerRegistry](k, "jobs.handlers")
	if err != nil {
		t.Fatalf("GetResource(jobs.handlers): %v", err)
	}
	if gotReg == nil {
		t.Fatal("expected non-nil handler registry from resource locator")
	}
}

func TestModule_ConfigDefaults(t *testing.T) {
	t.Parallel()
	m, _ := setupKernel(t, `
[modules]
jobs = true
`)

	cfg := m.cfg
	if want := runtime.NumCPU(); cfg.Workers != want {
		t.Errorf("Workers: got %d, want %d (runtime.NumCPU)", cfg.Workers, want)
	}
	if cfg.PollInterval != time.Second {
		t.Errorf("PollInterval: got %v, want 1s", cfg.PollInterval)
	}
	if cfg.LeaseDuration != 30*time.Second {
		t.Errorf("LeaseDuration: got %v, want 30s", cfg.LeaseDuration)
	}
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts: got %d, want 5", cfg.MaxAttempts)
	}
	if cfg.Backoff.Base != time.Second {
		t.Errorf("Backoff.Base: got %v, want 1s", cfg.Backoff.Base)
	}
	if cfg.Backoff.Max != time.Hour {
		t.Errorf("Backoff.Max: got %v, want 1h", cfg.Backoff.Max)
	}
	if cfg.Backoff.Jitter != 0.2 {
		t.Errorf("Backoff.Jitter: got %v, want 0.2", cfg.Backoff.Jitter)
	}
}

func TestModule_ConfigOverrides(t *testing.T) {
	t.Parallel()
	m, _ := setupKernel(t, `
[modules]
jobs = true

[jobs]
workers = 3
poll_interval = "100ms"
lease_duration = "5s"
max_attempts = 2
`)

	cfg := m.cfg
	if cfg.Workers != 3 {
		t.Errorf("Workers: got %d, want 3", cfg.Workers)
	}
	if cfg.PollInterval != 100*time.Millisecond {
		t.Errorf("PollInterval: got %v, want 100ms", cfg.PollInterval)
	}
	if cfg.LeaseDuration != 5*time.Second {
		t.Errorf("LeaseDuration: got %v, want 5s", cfg.LeaseDuration)
	}
	if cfg.MaxAttempts != 2 {
		t.Errorf("MaxAttempts: got %d, want 2", cfg.MaxAttempts)
	}
}

// — Submit —

func TestSubmit_UnknownHandlerReturnsError(t *testing.T) {
	t.Parallel()
	m, _ := setupKernel(t, `
[modules]
jobs = true
`)

	id, err := m.Submit(context.Background(), JobSpec{Type: "nope.handler"})
	if err == nil {
		t.Fatal("Submit(unknown handler): expected error, got nil")
	}
	if id != "" {
		t.Errorf("Submit(unknown handler): expected empty ID on error, got %q", id)
	}

	// And nothing should be enqueued.
	all, err := m.Store().List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 jobs in store, got %d", len(all))
	}
}

func TestSubmit_EncodesPayload(t *testing.T) {
	t.Parallel()
	m, _ := setupKernel(t, `
[modules]
jobs = true
`)

	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	type Payload struct {
		N int `json:"N"`
	}

	id, err := m.Submit(context.Background(), JobSpec{
		Type:    "h",
		Payload: Payload{N: 7},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Payload) == 0 {
		t.Fatal("expected non-empty payload")
	}

	var decoded Payload
	if err := json.Unmarshal(got.Payload, &decoded); err != nil {
		t.Fatalf("payload not valid JSON: %v (raw=%s)", err, string(got.Payload))
	}
	if decoded.N != 7 {
		t.Errorf("decoded.N: got %d, want 7", decoded.N)
	}
}

func TestSubmit_DefaultsAppliedFromConfig(t *testing.T) {
	t.Parallel()
	m, _ := setupKernel(t, `
[modules]
jobs = true

[jobs]
max_attempts = 7
`)

	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	before := time.Now()
	id, err := m.Submit(context.Background(), JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.MaxAttempts != 7 {
		t.Errorf("MaxAttempts: got %d, want 7 (from config)", got.MaxAttempts)
	}
	if got.Queue != "default" {
		t.Errorf("Queue: got %q, want %q", got.Queue, "default")
	}
	if got.RunAt.IsZero() {
		t.Error("RunAt: expected non-zero (now), got zero")
	}
	if delta := got.RunAt.Sub(before); delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("RunAt: default outside 5s window of now: delta=%v", delta)
	}
}

// — Worker —

func TestWorker_RunsHandler(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
poll_interval = "10ms"
`)

	called := make(chan struct{}, 1)
	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not invoked within 2s")
	}

	// Give the worker a moment to ack.
	if !waitForStatus(t, m, id, StatusDone, time.Second) {
		t.Fatalf("expected StatusDone")
	}
}

func TestWorker_RetryOnHandlerError(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
max_attempts = 5
poll_interval = "20ms"
lease_duration = "1s"
backoff_base = "10ms"
backoff_max = "100ms"
`)

	var calls atomic.Int32
	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error {
		n := calls.Add(1)
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !waitForStatus(t, m, id, StatusDone, 3*time.Second) {
		t.Fatalf("expected StatusDone within 3s, got something else")
	}

	if got := calls.Load(); got != 3 {
		t.Errorf("handler call count: got %d, want 3", got)
	}

	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Attempts != 3 {
		t.Errorf("Attempts: got %d, want 3", got.Attempts)
	}
}

func TestWorker_DeadAfterMaxAttempts(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
max_attempts = 2
poll_interval = "10ms"
lease_duration = "1s"
backoff_base = "5ms"
backoff_max = "50ms"
`)

	var calls atomic.Int32
	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error {
		calls.Add(1)
		return errors.New("always errors")
	})

	var deadFires atomic.Int32
	k.Hook("jobs.dead", 10, func(ctx context.Context) error {
		deadFires.Add(1)
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !waitForStatus(t, m, id, StatusFailed, 3*time.Second) {
		t.Fatalf("expected StatusFailed")
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("handler invocations: got %d, want 2", got)
	}

	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastError == "" {
		t.Error("expected non-empty LastError")
	}

	// Allow async hook firing time.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && deadFires.Load() < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := deadFires.Load(); got != 1 {
		t.Errorf("jobs.dead hook fires: got %d, want 1", got)
	}
}

func TestWorker_PanicRecovers(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
max_attempts = 1
poll_interval = "10ms"
lease_duration = "1s"
backoff_base = "5ms"
backoff_max = "50ms"
`)

	var paniced atomic.Bool
	m.Handlers().Handle("p", func(ctx context.Context, j *Job) error {
		paniced.Store(true)
		panic("kaboom")
	})

	okCalled := make(chan struct{}, 1)
	m.Handlers().Handle("ok", func(ctx context.Context, j *Job) error {
		select {
		case okCalled <- struct{}{}:
		default:
		}
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	panicID, err := m.Submit(ctx, JobSpec{Type: "p"})
	if err != nil {
		t.Fatalf("Submit(p): %v", err)
	}

	if !waitForStatus(t, m, panicID, StatusFailed, 3*time.Second) {
		t.Fatalf("expected StatusFailed for panicking job")
	}
	if !paniced.Load() {
		t.Error("handler did not run")
	}

	got, err := m.Store().Get(context.Background(), panicID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Contains([]byte(got.LastError), []byte("kaboom")) {
		t.Errorf("LastError: expected to contain %q, got %q", "kaboom", got.LastError)
	}

	// The worker pool must still process new jobs.
	if _, err := m.Submit(ctx, JobSpec{Type: "ok"}); err != nil {
		t.Fatalf("Submit(ok): %v", err)
	}
	select {
	case <-okCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("worker pool did not process job after panic")
	}
}

// — Cancel —

func TestModule_Cancel(t *testing.T) {
	t.Parallel()
	m, _ := setupKernel(t, `
[modules]
jobs = true
`)

	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	ctx := context.Background()

	// Future-scheduled job should sit pending.
	id, err := m.Submit(ctx, JobSpec{
		Type:  "h",
		RunAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if err := m.Cancel(ctx, id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := m.Store().Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != StatusCancelled {
		t.Errorf("Status: got %q, want %q", got.Status, StatusCancelled)
	}

	// Lease a separate job manually, then try to cancel it.
	leasedJob := &Job{Type: "h", Queue: "default"}
	if err := m.Store().Enqueue(ctx, leasedJob); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := m.Store().Lease(ctx, "test", []string{"default"}, time.Minute, 1); err != nil {
		t.Fatalf("Lease: %v", err)
	}
	err = m.Cancel(ctx, leasedJob.ID)
	if !errors.Is(err, ErrJobNotPending) {
		t.Fatalf("Cancel(leased): expected ErrJobNotPending, got %v", err)
	}
}

// — ExtendLease —

func TestModule_ExtendLease(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
poll_interval = "5ms"
lease_duration = "20ms"
max_attempts = 5
backoff_base = "5ms"
backoff_max = "50ms"
`)

	var attempts atomic.Int32

	m.Handlers().Handle("slow", func(ctx context.Context, j *Job) error {
		attempts.Add(1)
		// Extend our own lease so a second worker doesn't steal us mid-flight.
		if err := m.ExtendLease(ctx, j.ID, time.Second); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "slow"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !waitForStatus(t, m, id, StatusDone, 3*time.Second) {
		t.Fatalf("expected StatusDone")
	}

	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Attempts != 1 {
		t.Errorf("Attempts: got %d, want 1 (ExtendLease should prevent re-lease)", got.Attempts)
	}
	if a := attempts.Load(); a != 1 {
		t.Errorf("handler invocations: got %d, want 1", a)
	}
}

// — Hooks —

func TestHooks_BeforeAfterFireOnSuccess(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
poll_interval = "10ms"
`)

	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error { return nil })

	var before, after atomic.Int32
	var gotJobID, gotJobType atomic.Value

	k.Hook("jobs.before", 10, func(ctx context.Context) error {
		before.Add(1)
		if v := ctx.Value(JobIDKey); v != nil {
			if s, ok := v.(string); ok && s != "" {
				gotJobID.Store(s)
			}
		}
		if v := ctx.Value(JobTypeKey); v != nil {
			if s, ok := v.(string); ok && s != "" {
				gotJobType.Store(s)
			}
		}
		return nil
	})
	k.Hook("jobs.after", 10, func(ctx context.Context) error {
		after.Add(1)
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !waitForStatus(t, m, id, StatusDone, 3*time.Second) {
		t.Fatalf("expected StatusDone")
	}

	// Allow hook fire ordering to settle.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && (before.Load() < 1 || after.Load() < 1) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := before.Load(); got != 1 {
		t.Errorf("jobs.before: got %d, want 1", got)
	}
	if got := after.Load(); got != 1 {
		t.Errorf("jobs.after: got %d, want 1", got)
	}
	if v, _ := gotJobID.Load().(string); v != id {
		t.Errorf("jobs.before ctx job_id: got %q, want %q", v, id)
	}
	if v, _ := gotJobType.Load().(string); v != "h" {
		t.Errorf("jobs.before ctx job_type: got %q, want %q", v, "h")
	}
}

func TestHooks_ErrorFiresOnFailure(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
poll_interval = "10ms"
lease_duration = "1s"
max_attempts = 3
backoff_base = "5ms"
backoff_max = "50ms"
`)

	var calls atomic.Int32
	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error {
		if calls.Add(1) == 1 {
			return errors.New("first fails")
		}
		return nil
	})

	var errFires, retryFires, afterFires atomic.Int32
	k.Hook("jobs.error", 10, func(ctx context.Context) error {
		errFires.Add(1)
		return nil
	})
	k.HookResolve("jobs.retry", 10, func(ctx context.Context) (any, bool, error) {
		retryFires.Add(1)
		return nil, false, nil
	})
	k.Hook("jobs.after", 10, func(ctx context.Context) error {
		afterFires.Add(1)
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !waitForStatus(t, m, id, StatusDone, 3*time.Second) {
		t.Fatalf("expected StatusDone after retry")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if errFires.Load() >= 1 && retryFires.Load() >= 1 && afterFires.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := errFires.Load(); got != 1 {
		t.Errorf("jobs.error fires: got %d, want 1", got)
	}
	if got := retryFires.Load(); got != 1 {
		t.Errorf("jobs.retry resolve fires: got %d, want 1", got)
	}
	if got := afterFires.Load(); got != 1 {
		t.Errorf("jobs.after fires: got %d, want 1 (only on eventual success)", got)
	}
}

func TestHooks_RetryOverridesBackoff(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
poll_interval = "2ms"
lease_duration = "1s"
max_attempts = 5
backoff_base = "10s"
backoff_max = "1h"
`)

	var calls atomic.Int32
	firstAt := make(chan time.Time, 1)
	secondAt := make(chan time.Time, 1)

	m.Handlers().Handle("h", func(ctx context.Context, j *Job) error {
		n := calls.Add(1)
		now := time.Now()
		switch n {
		case 1:
			select {
			case firstAt <- now:
			default:
			}
			return errors.New("retry me")
		case 2:
			select {
			case secondAt <- now:
			default:
			}
			return nil
		}
		return nil
	})

	k.HookResolve("jobs.retry", 10, func(ctx context.Context) (any, bool, error) {
		return time.Now().Add(5 * time.Millisecond), true, nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "h"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !waitForStatus(t, m, id, StatusDone, 3*time.Second) {
		t.Fatalf("expected StatusDone after override retry")
	}

	var t1, t2 time.Time
	select {
	case t1 = <-firstAt:
	case <-time.After(2 * time.Second):
		t.Fatal("first attempt did not happen")
	}
	select {
	case t2 = <-secondAt:
	case <-time.After(2 * time.Second):
		t.Fatal("second attempt did not happen")
	}

	gap := t2.Sub(t1)
	if gap > 500*time.Millisecond {
		t.Errorf("retry gap = %v; expected ~5ms (with poll/lease slop ≤ 500ms); default backoff (10s) would have applied", gap)
	}
}

// — Drain —

func TestModule_GracefulDrain(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
workers = 4
poll_interval = "5ms"
`)

	m.Handlers().Handle("slow", func(ctx context.Context, j *Job) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var ids []string
	for i := 0; i < 3; i++ {
		id, err := m.Submit(ctx, JobSpec{Type: "slow"})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		ids = append(ids, id)
	}

	// Give the workers a moment to pick the jobs up.
	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := m.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	for _, id := range ids {
		got, err := m.Store().Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}
		if got.Status != StatusDone {
			t.Errorf("Get(%s): Status got %q, want %q", id, got.Status, StatusDone)
		}
	}
}

func TestModule_StopReturnsLeasedJobsToQueue(t *testing.T) {
	// Contract: with a too-short Stop deadline, Stop returns nil (best-effort
	// drain) AND any in-flight leased jobs are Nack'd back to pending so a
	// subsequent worker can pick them up.
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
workers = 1
poll_interval = "5ms"
lease_duration = "1s"
`)

	release := make(chan struct{})
	started := make(chan struct{}, 1)

	m.Handlers().Handle("block", func(ctx context.Context, j *Job) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	id, err := m.Submit(ctx, JobSpec{Type: "block"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never started")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	stopErr := m.Stop(stopCtx)
	if stopErr != nil {
		t.Errorf("Stop: got error %v, want nil under graceful-best-effort contract", stopErr)
	}

	// Release the blocked handler so it doesn't leak.
	close(release)

	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("Status: got %q, want %q (Nack'd back to pending)", got.Status, StatusPending)
	}
	if got.LockedBy != "" {
		t.Errorf("LockedBy: got %q, want \"\" (lease cleared)", got.LockedBy)
	}
}

// — Per-job logger —

func TestPerJobLogger_HasFields(t *testing.T) {
	m, k := setupKernel(t, `
[modules]
jobs = true

[jobs]
poll_interval = "10ms"
`)

	// Replace the kernel logger with one capturing JSON records.
	var buf bytes.Buffer
	var bufMu sync.Mutex
	captured := kernel.NewSlogLoggerFrom(slog.New(slog.NewJSONHandler(&lockedWriter{w: &buf, mu: &bufMu}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	k.SetLogger(captured)

	logged := make(chan struct{}, 1)
	m.Handlers().Handle("hh", func(ctx context.Context, j *Job) error {
		l := nslog.FromContext(ctx)
		if l == nil {
			l = k.Logger()
		}
		l.Info("handler running")
		select {
		case logged <- struct{}{}:
		default:
		}
		return nil
	})

	ctx := context.Background()
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = k.Stop(stopCtx)
	})

	id, err := m.Submit(ctx, JobSpec{Type: "hh"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case <-logged:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not log")
	}

	// Give the writer a moment to flush.
	time.Sleep(20 * time.Millisecond)

	bufMu.Lock()
	raw := buf.Bytes()
	bufMu.Unlock()

	var foundLine map[string]any
	for _, line := range bytesLines(raw) {
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if msg, _ := rec["msg"].(string); msg == "handler running" {
			foundLine = rec
			break
		}
	}
	if foundLine == nil {
		t.Fatalf("did not find handler log line in:\n%s", string(raw))
	}

	if v, _ := foundLine["job_id"].(string); v != id {
		t.Errorf("job_id field: got %q, want %q", v, id)
	}
	if v, _ := foundLine["job_type"].(string); v != "hh" {
		t.Errorf("job_type field: got %q, want %q", v, "hh")
	}
	if _, ok := foundLine["attempt"]; !ok {
		t.Error("expected attempt field on handler log line")
	}
}

// — helpers —

func waitForStatus(t *testing.T, m *Module, id string, want JobStatus, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got, err := m.Store().Get(context.Background(), id)
		if err == nil && got.Status == want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := m.Store().Get(context.Background(), id)
	if err != nil {
		t.Logf("Get(%s): %v", id, err)
		return false
	}
	t.Logf("waitForStatus: id=%s got=%q want=%q attempts=%d lastErr=%q", id, got.Status, want, got.Attempts, got.LastError)
	return false
}

func bytesLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

// lockedWriter is a small mutex-guarded io.Writer for tests that compare
// slog output across goroutines without races.
type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
