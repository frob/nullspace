package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"time"

	"github.com/frob/nullspace/kernel"
	datasql "github.com/frob/nullspace/module/data/sql"
)

// contextKey is the private type for job-related context values.
type contextKey string

const (
	// JobIDKey is the context key for the current job's ID.
	JobIDKey contextKey = "jobs.job_id"
	// JobTypeKey is the context key for the current job's type.
	JobTypeKey contextKey = "jobs.job_type"
	// jobErrorKey is the context key for the error from a failed handler.
	jobErrorKey contextKey = "jobs.error"
)

// Config holds the jobs module's runtime configuration.
type Config struct {
	Store         string
	Workers       int
	PollInterval  time.Duration
	LeaseDuration time.Duration
	MaxAttempts   int
	Backoff       BackoffConfig
}

// BackoffConfig holds exponential backoff parameters.
type BackoffConfig struct {
	Base   time.Duration
	Max    time.Duration
	Jitter float64
}

// rawConfig is a shadow struct used to decode the TOML section,
// because duration values arrive as strings via the JSON round-trip.
type rawConfig struct {
	Store         string  `json:"store"`
	Workers       int     `json:"workers"`
	PollInterval  string  `json:"poll_interval"`
	LeaseDuration string  `json:"lease_duration"`
	MaxAttempts   int     `json:"max_attempts"`
	BackoffBase   string  `json:"backoff_base"`
	BackoffMax    string  `json:"backoff_max"`
	BackoffJitter float64 `json:"backoff_jitter"`
}

// Module provides background job scheduling and execution.
// DefaultEnabled is false — apps opt in via [modules] jobs = true.
type Module struct {
	cfg      Config
	kernel   *kernel.Kernel
	store    Store
	registry *HandlerRegistry
	backoff  BackoffStrategy
	workers  *workerPool
}

// New creates a new jobs module.
func New() *Module {
	return &Module{registry: NewHandlerRegistry()}
}

// Name returns the module's unique name.
func (m *Module) Name() string { return "jobs" }

// Config declares the module's TOML section and defaults.
func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "jobs",
		Default: rawConfig{
			Store:         "memory",
			Workers:       0, // resolved to runtime.NumCPU() in Init
			PollInterval:  "1s",
			LeaseDuration: "30s",
			MaxAttempts:   5,
			BackoffBase:   "1s",
			BackoffMax:    "1h",
			BackoffJitter: 0.2,
		},
		DefaultEnabled: false,
	}
}

// Init wires the module into the kernel.
func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	var raw rawConfig
	if err := k.Config().Decode("jobs", &raw); err != nil {
		// fall back to bare defaults
		raw = rawConfig{
			Store:         "memory",
			PollInterval:  "1s",
			LeaseDuration: "30s",
			MaxAttempts:   5,
			BackoffBase:   "1s",
			BackoffMax:    "1h",
			BackoffJitter: 0.2,
		}
	}

	// Apply default for zero values.
	if raw.Store == "" {
		raw.Store = "memory"
	}
	if raw.MaxAttempts == 0 {
		raw.MaxAttempts = 5
	}
	if raw.BackoffJitter == 0 {
		raw.BackoffJitter = 0.2
	}

	// Parse duration strings.
	pollInterval, err := parseDurationDefault(raw.PollInterval, time.Second)
	if err != nil {
		return fmt.Errorf("jobs: poll_interval: %w", err)
	}
	leaseDuration, err := parseDurationDefault(raw.LeaseDuration, 30*time.Second)
	if err != nil {
		return fmt.Errorf("jobs: lease_duration: %w", err)
	}
	backoffBase, err := parseDurationDefault(raw.BackoffBase, time.Second)
	if err != nil {
		return fmt.Errorf("jobs: backoff_base: %w", err)
	}
	backoffMax, err := parseDurationDefault(raw.BackoffMax, time.Hour)
	if err != nil {
		return fmt.Errorf("jobs: backoff_max: %w", err)
	}

	workers := raw.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	m.cfg = Config{
		Store:         raw.Store,
		Workers:       workers,
		PollInterval:  pollInterval,
		LeaseDuration: leaseDuration,
		MaxAttempts:   raw.MaxAttempts,
		Backoff: BackoffConfig{
			Base:   backoffBase,
			Max:    backoffMax,
			Jitter: raw.BackoffJitter,
		},
	}

	// Build store.
	switch m.cfg.Store {
	case "memory":
		m.store = NewMemoryStore()
	case "sql":
		db, err := kernel.GetResource[*sql.DB](k, "db")
		if err != nil {
			return fmt.Errorf("jobs: store=sql requires data.sql module: %w", err)
		}

		var dsCfg struct{ Driver string }
		_ = k.Config().Decode("data.sql", &dsCfg)
		if dsCfg.Driver == "" {
			dsCfg.Driver = "sqlite"
		}

		reg, err := kernel.GetResource[*datasql.MigrationRegistry](k, "data.sql.migrations")
		if err != nil {
			return fmt.Errorf("jobs: migration registry not found: %w", err)
		}
		reg.Register("jobs", datasql.Migration{
			Version:     1,
			Description: "create jobs table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, JobsMigrationSQL)
				return err
			},
		})

		m.store = NewSQLStore(db, dsCfg.Driver)
	default:
		return fmt.Errorf("jobs: unknown store %q", m.cfg.Store)
	}

	if m.registry == nil {
		m.registry = NewHandlerRegistry()
	}
	m.backoff = ExponentialBackoff{
		Base:   m.cfg.Backoff.Base,
		Max:    m.cfg.Backoff.Max,
		Jitter: m.cfg.Backoff.Jitter,
	}

	k.Provide("jobs", m)
	k.Provide("jobs.handlers", m.registry)

	// Opportunistically register bridge handlers on any available TCP/IPC
	// transport. This is a no-op when those modules are absent.
	hookBridgeRegistration(k, m)

	return nil
}

// Start launches the worker pool.
func (m *Module) Start(ctx context.Context) error {
	m.workers = m.startWorkers(ctx)
	return nil
}

// Stop gracefully drains the worker pool.
func (m *Module) Stop(ctx context.Context) error {
	if m.workers != nil {
		m.workers.stop(ctx)
	}
	return nil
}

// Handlers returns the handler registry.
func (m *Module) Handlers() *HandlerRegistry { return m.registry }

// Store returns the backing job store.
func (m *Module) Store() Store { return m.store }

// Submit enqueues a new job and returns its ID.
func (m *Module) Submit(ctx context.Context, spec JobSpec) (string, error) {
	if spec.Type == "" {
		return "", fmt.Errorf("jobs: spec.Type must not be empty")
	}
	if !m.registry.Has(spec.Type) {
		return "", fmt.Errorf("jobs: unknown handler %q", spec.Type)
	}

	// Apply defaults.
	if spec.Queue == "" {
		spec.Queue = "default"
	}
	if spec.MaxAttempts == 0 {
		spec.MaxAttempts = m.cfg.MaxAttempts
	}
	if spec.RunAt.IsZero() {
		spec.RunAt = time.Now()
	}

	payload, err := EncodePayload(spec.Payload)
	if err != nil {
		return "", fmt.Errorf("jobs: encode payload: %w", err)
	}

	j := &Job{
		Type:        spec.Type,
		Queue:       spec.Queue,
		Payload:     payload,
		MaxAttempts: spec.MaxAttempts,
		RunAt:       spec.RunAt,
	}

	if err := m.store.Enqueue(ctx, j); err != nil {
		return "", fmt.Errorf("jobs: enqueue: %w", err)
	}

	return j.ID, nil
}

// Cancel cancels a pending job.
func (m *Module) Cancel(ctx context.Context, jobID string) error {
	return m.store.Cancel(ctx, jobID)
}

// ExtendLease extends the lease on a currently-leased job by dur.
func (m *Module) ExtendLease(ctx context.Context, jobID string, dur time.Duration) error {
	return m.store.ExtendLease(ctx, jobID, time.Now().Add(dur))
}

// parseDurationDefault parses s as a duration; returns def if s is empty.
func parseDurationDefault(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def, err
	}
	return d, nil
}
