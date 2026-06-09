package jobs

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	datasql "github.com/frob/nullspace/module/data/sql"
)

// setupKernelWithSQL boots a kernel with the data.sql module + jobs module
// configured to use the SQL store. The data.sql module exposes a sqlite DB
// at dbPath, and on kernel.after_init the jobs module's migration is run.
func setupKernelWithSQL(t *testing.T, dbPath string) *kernel.Kernel {
	t.Helper()

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "nullspace.toml")

	toml := `
[modules]
"data.sql" = true
jobs = true

[data.sql]
driver = "sqlite"
dsn = "` + dbPath + `"

[jobs]
store = "sql"
`
	if err := os.WriteFile(tomlPath, []byte(toml), 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	k.Use(routing.New())
	k.Use(datasql.New())
	k.Use(New())

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return k
}

// TestSQLModuleRegistersMigration boots a kernel with data.sql + jobs (store=sql)
// and verifies that after Init the jobs table exists in the database.
func TestSQLModuleRegistersMigration(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "kernel_jobs.db")
	k := setupKernelWithSQL(t, dbPath)

	db, err := kernel.GetResource[*sql.DB](k, "db")
	if err != nil {
		t.Fatalf("GetResource(db): %v", err)
	}

	// The jobs table must exist after kernel.after_init has fired.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='jobs'`).Scan(&n); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected jobs table to exist after kernel init, got count=%d", n)
	}

	// And the migration record should be present for the jobs module.
	var migN int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _migrations WHERE module = 'jobs'`).Scan(&migN); err != nil {
		t.Fatalf("query _migrations: %v", err)
	}
	if migN < 1 {
		t.Errorf("expected at least 1 jobs migration record, got %d", migN)
	}
}

// TestSQLModule_StoreIsSQLStore confirms the jobs module wires its Store to
// the SQL store (not memory) when [jobs] store = "sql".
func TestSQLModule_StoreIsSQLStore(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "kernel_jobs_store.db")
	k := setupKernelWithSQL(t, dbPath)

	jobsMod, err := kernel.GetResource[*Module](k, "jobs")
	if err != nil {
		t.Fatalf("GetResource(jobs): %v", err)
	}

	if _, ok := jobsMod.Store().(*SQLStore); !ok {
		t.Fatalf("Store: got %T, want *SQLStore", jobsMod.Store())
	}
}
