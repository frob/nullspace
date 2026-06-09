-- Jobs table. Column types chosen for SQLite/Postgres compatibility:
--   TEXT  works on both; Postgres BLOB would need BYTEA — tests use SQLite only.
--   Timestamps are stored as RFC3339Nano text strings.
--   locked_until / locked_by / last_error default to '' (empty string = no value).
-- Postgres note: BLOB is not valid Postgres syntax; if adding Postgres support,
-- use a driver-specific migration file or map BLOB→BYTEA at runtime.

CREATE TABLE IF NOT EXISTS jobs (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL,
    queue         TEXT NOT NULL DEFAULT 'default',
    payload       BLOB,
    status        TEXT NOT NULL DEFAULT 'pending',
    attempts      INTEGER NOT NULL DEFAULT 0,
    max_attempts  INTEGER NOT NULL DEFAULT 5,
    run_at        TEXT NOT NULL,
    locked_until  TEXT NOT NULL DEFAULT '',
    locked_by     TEXT NOT NULL DEFAULT '',
    last_error    TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_queue_runat ON jobs (status, queue, run_at);
CREATE INDEX IF NOT EXISTS idx_jobs_locked_until ON jobs (locked_until);
