package jobs

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"
)

//go:embed store_sql_migrations/0001_jobs.sql
var JobsMigrationSQL string

// SQLStore is a SQL-backed Store. It supports SQLite and Postgres.
// Concurrent safety for SQLite relies on BEGIN IMMEDIATE transactions in Lease;
// for Postgres, FOR UPDATE SKIP LOCKED achieves the same effect server-side.
type SQLStore struct {
	db     *sql.DB
	driver string // "sqlite" | "postgres"
}

// NewSQLStore returns a SQLStore backed by db. driver must be "sqlite" or "postgres".
func NewSQLStore(db *sql.DB, driver string) *SQLStore {
	return &SQLStore{db: db, driver: driver}
}

// encodeTime serialises a time.Time to RFC3339Nano with monotonic stripped.
// Zero time is encoded as an empty string.
func encodeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(0).Format(time.RFC3339Nano)
}

// decodeTime parses a stored RFC3339Nano string. Empty string → zero time.
func decodeTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		// Fallback: try without sub-second precision.
		t, err = time.Parse(time.RFC3339, s)
	}
	return t.UTC(), err
}

// scanJob reads a full job row from rows in column-declaration order.
func scanJob(rows *sql.Rows) (*Job, error) {
	var (
		j                                                    Job
		payloadBytes                                         []byte
		runAtStr, lockedUntilStr, createdAtStr, updatedAtStr string
	)
	err := rows.Scan(
		&j.ID,
		&j.Type,
		&j.Queue,
		&payloadBytes,
		(*string)(&j.Status),
		&j.Attempts,
		&j.MaxAttempts,
		&runAtStr,
		&lockedUntilStr,
		&j.LockedBy,
		&j.LastError,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}
	j.Payload = payloadBytes

	if j.RunAt, err = decodeTime(runAtStr); err != nil {
		return nil, fmt.Errorf("parse run_at %q: %w", runAtStr, err)
	}
	if j.LockedUntil, err = decodeTime(lockedUntilStr); err != nil {
		return nil, fmt.Errorf("parse locked_until %q: %w", lockedUntilStr, err)
	}
	if j.CreatedAt, err = decodeTime(createdAtStr); err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAtStr, err)
	}
	if j.UpdatedAt, err = decodeTime(updatedAtStr); err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAtStr, err)
	}
	return &j, nil
}

// scanJobRow is like scanJob but for a single *sql.Row.
func scanJobRow(row *sql.Row) (*Job, error) {
	var (
		j                                                    Job
		payloadBytes                                         []byte
		runAtStr, lockedUntilStr, createdAtStr, updatedAtStr string
	)
	err := row.Scan(
		&j.ID,
		&j.Type,
		&j.Queue,
		&payloadBytes,
		(*string)(&j.Status),
		&j.Attempts,
		&j.MaxAttempts,
		&runAtStr,
		&lockedUntilStr,
		&j.LockedBy,
		&j.LastError,
		&createdAtStr,
		&updatedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	j.Payload = payloadBytes

	if j.RunAt, err = decodeTime(runAtStr); err != nil {
		return nil, fmt.Errorf("parse run_at %q: %w", runAtStr, err)
	}
	if j.LockedUntil, err = decodeTime(lockedUntilStr); err != nil {
		return nil, fmt.Errorf("parse locked_until %q: %w", lockedUntilStr, err)
	}
	if j.CreatedAt, err = decodeTime(createdAtStr); err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAtStr, err)
	}
	if j.UpdatedAt, err = decodeTime(updatedAtStr); err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAtStr, err)
	}
	return &j, nil
}

// Enqueue inserts a new job, filling in defaults (same as MemoryStore).
func (s *SQLStore) Enqueue(ctx context.Context, j *Job) error {
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

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs
		    (id, type, queue, payload, status, attempts, max_attempts,
		     run_at, locked_until, locked_by, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Type, j.Queue, j.Payload,
		string(j.Status), j.Attempts, j.MaxAttempts,
		encodeTime(j.RunAt),
		encodeTime(j.LockedUntil),
		j.LockedBy, j.LastError,
		encodeTime(j.CreatedAt),
		encodeTime(j.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("jobs: enqueue: %w", err)
	}
	return nil
}

// Lease claims up to max pending (or expired-leased) jobs from queues.
func (s *SQLStore) Lease(ctx context.Context, workerID string, queues []string, leaseFor time.Duration, max int) ([]*Job, error) {
	if len(queues) == 0 {
		return nil, nil
	}
	if s.driver == "postgres" {
		return s.leasePostgres(ctx, workerID, queues, leaseFor, max)
	}
	return s.leaseSQLite(ctx, workerID, queues, leaseFor, max)
}

// leaseSQLite uses a BEGIN IMMEDIATE transaction + RETURNING to atomically
// claim jobs in SQLite. SQLite serialises writers, so an IMMEDIATE (serializable)
// transaction prevents two concurrent goroutines from selecting the same rows.
// On SQLITE_BUSY, the call retries with exponential backoff so that callers
// sharing multiple *sql.DB handles to the same file still make progress.
func (s *SQLStore) leaseSQLite(ctx context.Context, workerID string, queues []string, leaseFor time.Duration, max int) ([]*Job, error) {
	now := time.Now()
	nowStr := encodeTime(now)
	until := now.Add(leaseFor)
	untilStr := encodeTime(until)

	placeholders := make([]string, len(queues))
	queueArgs := make([]any, len(queues))
	for i, q := range queues {
		placeholders[i] = "?"
		queueArgs[i] = q
	}
	queueIn := strings.Join(placeholders, ",")

	// Args must match the query's ? positions in order:
	//   SET: status, locked_by, locked_until, updated_at
	//   subquery WHERE: locked_until filter, run_at filter, then queue values, then LIMIT
	updateArgs := make([]any, 0, 7+len(queueArgs))
	updateArgs = append(updateArgs,
		string(StatusLeased), workerID, untilStr, nowStr, // SET clause
		nowStr, nowStr, // subquery: locked_until<=?, run_at<=?
	)
	updateArgs = append(updateArgs, queueArgs...) // subquery: queue IN (...)
	updateArgs = append(updateArgs, max)          // LIMIT

	// The subquery selects IDs, the outer UPDATE claims them.
	query := fmt.Sprintf(`
		UPDATE jobs SET
		    status       = ?,
		    locked_by    = ?,
		    locked_until = ?,
		    updated_at   = ?
		WHERE id IN (
		    SELECT id FROM jobs
		    WHERE (
		        status = 'pending'
		        OR (status = 'leased' AND locked_until != '' AND locked_until <= ?)
		    )
		    AND run_at <= ?
		    AND queue IN (%s)
		    ORDER BY run_at ASC
		    LIMIT ?
		)
		RETURNING id, type, queue, payload, status, attempts, max_attempts,
		          run_at, locked_until, locked_by, last_error, created_at, updated_at
	`, queueIn)

	// Retry loop: multiple *sql.DB handles to the same file can race on the
	// write lock. SQLITE_BUSY is transient — a brief sleep and retry is safe.
	const maxRetries = 20
	delays := [...]time.Duration{1, 2, 3, 5, 8, 13, 21, 34} // ms, Fibonacci-ish
	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err := s.execLeaseSQLite(ctx, query, updateArgs)
		if err == nil {
			return result, nil
		}
		if !isSQLiteBusy(err) {
			return nil, err
		}
		// Transient lock contention — back off and retry.
		delay := delays[attempt%len(delays)] * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("jobs: lease sqlite: too many retries (SQLITE_BUSY)")
}

// execLeaseSQLite runs one attempt of the SQLite lease transaction.
func (s *SQLStore) execLeaseSQLite(ctx context.Context, query string, args []any) ([]*Job, error) {
	// LevelSerializable → BEGIN IMMEDIATE on modernc.org/sqlite, which acquires
	// a write lock upfront and prevents two concurrent transactions from
	// selecting the same rows before either upgrades to write.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("jobs: lease begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("jobs: lease query: %w", err)
	}

	var result []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("jobs: lease scan: %w", err)
		}
		result = append(result, j)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("jobs: lease rows: %w", err)
	}
	rows.Close()

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("jobs: lease commit: %w", err)
	}
	return result, nil
}

// isSQLiteBusy reports whether err is a transient SQLite lock-contention error.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "database is locked")
}

// leasePostgres uses FOR UPDATE SKIP LOCKED for atomic multi-connection leasing.
func (s *SQLStore) leasePostgres(ctx context.Context, workerID string, queues []string, leaseFor time.Duration, max int) ([]*Job, error) {
	now := time.Now()
	nowStr := encodeTime(now)
	until := now.Add(leaseFor)
	untilStr := encodeTime(until)

	placeholders := make([]string, len(queues))
	args := make([]any, len(queues))
	for i, q := range queues {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = q
	}
	queueIn := strings.Join(placeholders, ",")
	base := len(queues) + 1

	args = append(args,
		nowStr,               // locked_until filter
		nowStr,               // run_at filter
		max,                  // LIMIT
		string(StatusLeased), // status update
		workerID,             // locked_by update
		untilStr,             // locked_until update
		nowStr,               // updated_at update
	)

	query := fmt.Sprintf(`
		WITH leased AS (
		    SELECT id FROM jobs
		    WHERE (
		        status = 'pending'
		        OR (status = 'leased' AND locked_until != '' AND locked_until <= $%d)
		    )
		    AND run_at <= $%d
		    AND queue IN (%s)
		    ORDER BY run_at ASC
		    LIMIT $%d
		    FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs SET
		    status       = $%d,
		    locked_by    = $%d,
		    locked_until = $%d,
		    updated_at   = $%d
		WHERE id IN (SELECT id FROM leased)
		RETURNING id, type, queue, payload, status, attempts, max_attempts,
		          run_at, locked_until, locked_by, last_error, created_at, updated_at
	`, base, base+1, queueIn, base+2, base+3, base+4, base+5, base+6)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("jobs: lease (postgres): %w", err)
	}
	defer rows.Close()

	var result []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("jobs: lease scan: %w", err)
		}
		result = append(result, j)
	}
	return result, rows.Err()
}

// Ack marks a leased job as done and increments Attempts.
func (s *SQLStore) Ack(ctx context.Context, jobID string) error {
	now := encodeTime(time.Now())
	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status='done', attempts=attempts+1, updated_at=? WHERE id=?`,
		now, jobID,
	)
	if err != nil {
		return fmt.Errorf("jobs: ack: %w", err)
	}
	return requireOneRow(res, ErrJobNotFound)
}

// Nack returns a leased job to pending, increments Attempts, and sets LastError.
func (s *SQLStore) Nack(ctx context.Context, jobID string, retryAt time.Time, lastErr string) error {
	now := encodeTime(time.Now())
	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET
		    status='pending', attempts=attempts+1, last_error=?,
		    run_at=?, locked_by='', locked_until='', updated_at=?
		 WHERE id=?`,
		lastErr, encodeTime(retryAt), now, jobID,
	)
	if err != nil {
		return fmt.Errorf("jobs: nack: %w", err)
	}
	return requireOneRow(res, ErrJobNotFound)
}

// Fail marks a job as permanently failed.
func (s *SQLStore) Fail(ctx context.Context, jobID string, lastErr string) error {
	now := encodeTime(time.Now())
	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status='failed', last_error=?, updated_at=? WHERE id=?`,
		lastErr, now, jobID,
	)
	if err != nil {
		return fmt.Errorf("jobs: fail: %w", err)
	}
	return requireOneRow(res, ErrJobNotFound)
}

// Cancel cancels a pending job. Returns ErrJobNotPending if not pending, ErrJobNotFound if missing.
func (s *SQLStore) Cancel(ctx context.Context, jobID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("jobs: cancel begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=?`, jobID).Scan(&status)
	if err == sql.ErrNoRows {
		return ErrJobNotFound
	}
	if err != nil {
		return fmt.Errorf("jobs: cancel query: %w", err)
	}
	if JobStatus(status) != StatusPending {
		return ErrJobNotPending
	}

	now := encodeTime(time.Now())
	if _, err := tx.ExecContext(ctx,
		`UPDATE jobs SET status='cancelled', updated_at=? WHERE id=?`,
		now, jobID,
	); err != nil {
		return fmt.Errorf("jobs: cancel update: %w", err)
	}
	return tx.Commit()
}

// Get retrieves a job by ID.
func (s *SQLStore) Get(ctx context.Context, jobID string) (*Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, type, queue, payload, status, attempts, max_attempts,
		        run_at, locked_until, locked_by, last_error, created_at, updated_at
		 FROM jobs WHERE id=?`,
		jobID,
	)
	j, err := scanJobRow(row)
	if err != nil {
		return nil, fmt.Errorf("jobs: get: %w", err)
	}
	return j, nil
}

// List returns jobs matching the filter.
func (s *SQLStore) List(ctx context.Context, filter ListFilter) ([]*Job, error) {
	var sb strings.Builder
	sb.WriteString(
		`SELECT id, type, queue, payload, status, attempts, max_attempts,
		        run_at, locked_until, locked_by, last_error, created_at, updated_at
		 FROM jobs`,
	)

	var args []any
	var clauses []string

	if len(filter.Statuses) > 0 {
		phs := make([]string, len(filter.Statuses))
		for i, st := range filter.Statuses {
			phs[i] = "?"
			args = append(args, string(st))
		}
		clauses = append(clauses, "status IN ("+strings.Join(phs, ",")+")")
	}
	if filter.Queue != "" {
		clauses = append(clauses, "queue=?")
		args = append(args, filter.Queue)
	}
	if filter.Type != "" {
		clauses = append(clauses, "type=?")
		args = append(args, filter.Type)
	}

	if len(clauses) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(clauses, " AND "))
	}
	if filter.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("jobs: list: %w", err)
	}
	defer rows.Close()

	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("jobs: list scan: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ExtendLease updates the LockedUntil on a currently-leased job.
func (s *SQLStore) ExtendLease(ctx context.Context, jobID string, until time.Time) error {
	now := encodeTime(time.Now())
	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET locked_until=?, updated_at=? WHERE id=? AND status='leased'`,
		encodeTime(until), now, jobID,
	)
	if err != nil {
		return fmt.Errorf("jobs: extend lease: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("jobs: extend lease rows affected: %w", err)
	}
	if n == 0 {
		// Distinguish missing vs. wrong status.
		var status string
		err := s.db.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id=?`, jobID).Scan(&status)
		if err == sql.ErrNoRows {
			return ErrJobNotFound
		}
		if err != nil {
			return fmt.Errorf("jobs: extend lease status check: %w", err)
		}
		return ErrJobNotLeased
	}
	return nil
}

// requireOneRow returns sentinel if RowsAffected == 0.
func requireOneRow(res sql.Result, sentinel error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sentinel
	}
	return nil
}
