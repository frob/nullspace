package sql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/frob/nullspace/kernel"
)

func setupMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "migrate_test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrationRegistryRun(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	reg := NewMigrationRegistry()
	reg.Register("app", Migration{
		Version:     1,
		Description: "create users table",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
			return err
		},
	})

	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify the table was created.
	_, err := db.ExecContext(ctx, `INSERT INTO users (name) VALUES ('alice')`)
	if err != nil {
		t.Fatalf("insert into users: %v", err)
	}

	// Verify migration was recorded.
	var count int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations WHERE module = 'app' AND version = 1`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 migration record, got %d", count)
	}
}

func TestMigrationsRunOnce(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	calls := 0
	reg := NewMigrationRegistry()
	reg.Register("app", Migration{
		Version:     1,
		Description: "counted migration",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			calls++
			_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS counter (id INTEGER PRIMARY KEY)`)
			return err
		},
	})

	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected migration to run once, ran %d times", calls)
	}
}

func TestMigrationVersionOrder(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	var order []int
	reg := NewMigrationRegistry()

	// Register out of order.
	reg.Register("app",
		Migration{
			Version: 3, Description: "v3",
			Up: func(ctx context.Context, tx *sql.Tx) error { order = append(order, 3); return nil },
		},
		Migration{
			Version: 1, Description: "v1",
			Up: func(ctx context.Context, tx *sql.Tx) error { order = append(order, 1); return nil },
		},
		Migration{
			Version: 2, Description: "v2",
			Up: func(ctx context.Context, tx *sql.Tx) error { order = append(order, 2); return nil },
		},
	)

	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected [1 2 3], got %v", order)
	}
}

func TestMultipleModules(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	reg := NewMigrationRegistry()
	reg.Register("session", Migration{
		Version: 1, Description: "create sessions",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `CREATE TABLE sessions (id TEXT PRIMARY KEY)`)
			return err
		},
	})
	reg.Register("app", Migration{
		Version: 1, Description: "create posts",
		Up: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `CREATE TABLE posts (id INTEGER PRIMARY KEY)`)
			return err
		},
	})

	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Both tables should exist.
	for _, table := range []string{"sessions", "posts"} {
		var n int
		err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&n)
		if err != nil {
			t.Fatalf("table %s not created: %v", table, err)
		}
	}

	// Both modules should have records in _migrations.
	var count int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations`).Scan(&count)
	if count != 2 {
		t.Fatalf("expected 2 migration records, got %d", count)
	}
}

func TestMultipleModulesRunInRegistrationOrder(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	var order []string
	reg := NewMigrationRegistry()

	reg.Register("beta", Migration{
		Version: 1, Description: "beta v1",
		Up: func(ctx context.Context, tx *sql.Tx) error { order = append(order, "beta"); return nil },
	})
	reg.Register("alpha", Migration{
		Version: 1, Description: "alpha v1",
		Up: func(ctx context.Context, tx *sql.Tx) error { order = append(order, "alpha"); return nil },
	})

	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Should run in registration order, not alphabetical.
	if len(order) != 2 || order[0] != "beta" || order[1] != "alpha" {
		t.Fatalf("expected [beta alpha], got %v", order)
	}
}

func TestMigrationFailureRollsBack(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	reg := NewMigrationRegistry()
	reg.Register("app",
		Migration{
			Version: 1, Description: "create table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `CREATE TABLE things (id INTEGER PRIMARY KEY)`)
				return err
			},
		},
		Migration{
			Version: 2, Description: "this will fail",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				return fmt.Errorf("intentional failure")
			},
		},
	)

	err := reg.Run(ctx, db)
	if err == nil {
		t.Fatal("expected error from failed migration")
	}

	// v1 should have been applied (it succeeded before v2 failed).
	var count int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations WHERE module = 'app' AND version = 1`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected v1 to be applied, got %d records", count)
	}

	// v2 should NOT be recorded.
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations WHERE module = 'app' AND version = 2`).Scan(&count)
	if count != 0 {
		t.Fatalf("expected v2 to not be applied, got %d records", count)
	}
}

func TestConcurrentRegister(t *testing.T) {
	reg := NewMigrationRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			reg.Register(fmt.Sprintf("mod%d", v), Migration{
				Version: 1, Description: fmt.Sprintf("migration from goroutine %d", v),
				Up: func(ctx context.Context, tx *sql.Tx) error { return nil },
			})
		}(i)
	}
	wg.Wait()

	db := setupMigrationDB(t)
	ctx := context.Background()

	if err := reg.Run(ctx, db); err != nil {
		t.Fatalf("Run after concurrent registration: %v", err)
	}

	var count int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations`).Scan(&count)
	if count != 10 {
		t.Fatalf("expected 10 migration records, got %d", count)
	}
}

func TestMigrationRegistryViaKernelLifecycle(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "lifecycle.db")

	tomlPath := filepath.Join(dir, "nullspace.toml")
	os.WriteFile(tomlPath, []byte(`
[modules]
"data.sql" = true

[data.sql]
driver = "sqlite"
dsn = "`+dbPath+`"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	m := New()
	k.Use(m)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Retrieve the migration registry from the service locator.
	reg, err := kernel.GetResource[*MigrationRegistry](k, "data.sql.migrations")
	if err != nil {
		t.Fatalf("expected data.sql.migrations resource: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil migration registry")
	}
}
