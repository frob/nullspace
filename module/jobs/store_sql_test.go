package jobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openSQLiteFileDB opens a fresh sqlite database at a per-test file path and
// applies the embedded jobs migration directly (no kernel wiring). It returns
// the opened DB and a cleanup function that closes it.
func openSQLiteFileDB(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jobs_test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := applyJobsMigration(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("apply migration: %v", err)
	}
	return db, path, func() { _ = db.Close() }
}

// applyJobsMigration runs the embedded migration SQL on the given DB. The
// embedded SQL is exposed by store_sql.go (RED state until GREEN ships).
func applyJobsMigration(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, JobsMigrationSQL)
	return err
}

// TestSQLStoreConformance_SQLite runs the full Store contract suite against
// the SQLite-backed SQLStore. Each subtest gets a fresh DB file under t.TempDir.
func TestSQLStoreConformance_SQLite(t *testing.T) {
	t.Parallel()

	factory := func(t *testing.T) (Store, func()) {
		t.Helper()
		db, _, closeDB := openSQLiteFileDB(t)
		store := NewSQLStore(db, "sqlite")
		return store, closeDB
	}

	RunStoreConformance(t, factory)
}

// TestSQLStore_RoundTripJobFields enqueues a job with every field populated
// and asserts Get returns identical values. Catches column-mapping bugs.
func TestSQLStore_RoundTripJobFields(t *testing.T) {
	t.Parallel()

	db, _, cleanup := openSQLiteFileDB(t)
	t.Cleanup(cleanup)

	store := NewSQLStore(db, "sqlite")
	ctx := context.Background()

	runAt := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Microsecond)
	j := &Job{
		Type:        "send_email",
		Queue:       "high",
		Payload:     []byte(`{"to":"a@b","subject":"hi"}`),
		MaxAttempts: 9,
		RunAt:       runAt,
		LastError:   "previous attempt failed",
	}
	if err := store.Enqueue(ctx, j); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if j.ID == "" {
		t.Fatal("Enqueue: expected ID to be assigned")
	}

	got, err := store.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != j.ID {
		t.Errorf("ID: got %q, want %q", got.ID, j.ID)
	}
	if got.Type != j.Type {
		t.Errorf("Type: got %q, want %q", got.Type, j.Type)
	}
	if got.Queue != j.Queue {
		t.Errorf("Queue: got %q, want %q", got.Queue, j.Queue)
	}
	if string(got.Payload) != string(j.Payload) {
		t.Errorf("Payload: got %q, want %q", got.Payload, j.Payload)
	}
	if got.MaxAttempts != j.MaxAttempts {
		t.Errorf("MaxAttempts: got %d, want %d", got.MaxAttempts, j.MaxAttempts)
	}
	if !got.RunAt.Equal(runAt) {
		t.Errorf("RunAt: got %v, want %v", got.RunAt, runAt)
	}
	if got.LastError != j.LastError {
		t.Errorf("LastError: got %q, want %q", got.LastError, j.LastError)
	}
	if got.Status != StatusPending {
		t.Errorf("Status: got %q, want %q", got.Status, StatusPending)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt: expected non-zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt: expected non-zero")
	}
}

// TestSQLStore_MigrationCreatesTable opens a fresh sqlite DB, applies the
// embedded migration, and then queries sqlite_master to assert that the jobs
// table and the two indexes exist.
func TestSQLStore_MigrationCreatesTable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "schema.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := applyJobsMigration(context.Background(), db); err != nil {
		t.Fatalf("applyJobsMigration: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='jobs'`).Scan(&n); err != nil {
		t.Fatalf("query jobs table: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected jobs table to exist, got count=%d", n)
	}

	for _, idx := range []string{
		"idx_jobs_status_queue_runat",
		"idx_jobs_locked_until",
	} {
		var got int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&got); err != nil {
			t.Fatalf("query index %s: %v", idx, err)
		}
		if got != 1 {
			t.Errorf("expected index %q to exist, got count=%d", idx, got)
		}
	}
}

// TestSQLStore_LeasePreventsDoubleDelivery_Concurrent opens N parallel DB
// handles to the SAME on-disk SQLite file and races Lease calls. Asserts that
// at most one worker leases the job. Catches transaction-isolation bugs.
func TestSQLStore_LeasePreventsDoubleDelivery_Concurrent(t *testing.T) {
	t.Parallel()

	// Shared on-disk file so each DB handle sees the same data.
	path := filepath.Join(t.TempDir(), "shared.db")

	openOne := func() *sql.DB {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open: %v", err)
		}
		return db
	}

	setupDB := openOne()
	t.Cleanup(func() { _ = setupDB.Close() })
	if err := applyJobsMigration(context.Background(), setupDB); err != nil {
		t.Fatalf("applyJobsMigration: %v", err)
	}
	setupStore := NewSQLStore(setupDB, "sqlite")

	j := &Job{Type: "t", Queue: "q1"}
	if err := setupStore.Enqueue(context.Background(), j); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	const N = 10
	stores := make([]*SQLStore, N)
	for i := 0; i < N; i++ {
		db := openOne()
		t.Cleanup(func() { _ = db.Close() })
		stores[i] = NewSQLStore(db, "sqlite")
	}

	var wg sync.WaitGroup
	wg.Add(N)
	start := make(chan struct{})
	var winners atomic.Int32
	var empties atomic.Int32
	errs := make(chan error, N)

	for i := 0; i < N; i++ {
		s := stores[i]
		workerID := i
		go func() {
			defer wg.Done()
			<-start
			leased, err := s.Lease(context.Background(), workerName(workerID), []string{"q1"}, 30*time.Second, 1)
			if err != nil {
				errs <- err
				return
			}
			switch len(leased) {
			case 1:
				winners.Add(1)
			case 0:
				empties.Add(1)
			default:
				errs <- errTooMany(len(leased))
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("Lease (concurrent across connections): %v", err)
	}
	if w := winners.Load(); w != 1 {
		t.Errorf("winners: got %d, want 1", w)
	}
	if e := empties.Load(); e != N-1 {
		t.Errorf("empties: got %d, want %d", e, N-1)
	}
}

// TestSQLStore_ExtendLeaseUpdatesRow enqueues, leases, then ExtendLeases the
// job for ~2s, and queries the row directly via SQL to assert locked_until
// landed inside a small window of now+2s.
func TestSQLStore_ExtendLeaseUpdatesRow(t *testing.T) {
	t.Parallel()

	db, _, cleanup := openSQLiteFileDB(t)
	t.Cleanup(cleanup)

	store := NewSQLStore(db, "sqlite")
	ctx := context.Background()

	j := &Job{Type: "t", Queue: "q1"}
	if err := store.Enqueue(ctx, j); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	leased, err := store.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 1)
	if err != nil {
		t.Fatalf("Lease: %v", err)
	}
	if len(leased) != 1 {
		t.Fatalf("Lease: got %d, want 1", len(leased))
	}

	target := time.Now().Add(2 * time.Second).UTC()
	if err := store.ExtendLease(ctx, j.ID, target); err != nil {
		t.Fatalf("ExtendLease: %v", err)
	}

	// Read locked_until directly. The column may be stored as text or unix —
	// try text first, then fall back to int64.
	var lockedUntil time.Time
	var rawText string
	if err := db.QueryRowContext(ctx, `SELECT locked_until FROM jobs WHERE id = ?`, j.ID).Scan(&rawText); err == nil && rawText != "" {
		// Try RFC3339Nano, RFC3339, then assume the SQLStore wrote whatever it wrote.
		if parsed, perr := time.Parse(time.RFC3339Nano, rawText); perr == nil {
			lockedUntil = parsed
		} else if parsed, perr := time.Parse(time.RFC3339, rawText); perr == nil {
			lockedUntil = parsed
		} else {
			t.Fatalf("could not parse locked_until %q: %v", rawText, perr)
		}
	} else {
		var unixSec int64
		if err := db.QueryRowContext(ctx, `SELECT locked_until FROM jobs WHERE id = ?`, j.ID).Scan(&unixSec); err != nil {
			t.Fatalf("query locked_until: %v", err)
		}
		lockedUntil = time.Unix(unixSec, 0).UTC()
	}

	if delta := lockedUntil.Sub(target); delta < -2*time.Second || delta > 2*time.Second {
		t.Errorf("locked_until: got %v, want within 2s of %v (delta=%v)", lockedUntil, target, delta)
	}
}
