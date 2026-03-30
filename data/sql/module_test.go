package sql

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/frob/nullspace/kernel"
)

func setupSQLModule(t *testing.T) (*Module, *kernel.Kernel) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	tomlPath := filepath.Join(t.TempDir(), "nullspace.toml")
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

	return m, k
}

func TestModuleName(t *testing.T) {
	m := New()
	if m.Name() != "data.sql" {
		t.Fatalf("expected 'data.sql', got %q", m.Name())
	}
}

func TestModuleDefaultDisabled(t *testing.T) {
	m := New()
	cfg := m.Config()
	if cfg.DefaultEnabled {
		t.Fatal("SQL module should default to disabled")
	}
}

func TestSQLiteConnection(t *testing.T) {
	m, _ := setupSQLModule(t)

	if m.DB() == nil {
		t.Fatal("expected non-nil DB")
	}

	ctx := context.Background()
	if err := m.Healthy(ctx); err != nil {
		t.Fatalf("Healthy: %v", err)
	}
}

func TestSQLiteStartVerifiesConnection(t *testing.T) {
	m, _ := setupSQLModule(t)

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestSQLiteExecAndQuery(t *testing.T) {
	m, _ := setupSQLModule(t)
	ctx := context.Background()

	// Create a table.
	_, err := m.Exec(ctx, `CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("Exec CREATE: %v", err)
	}

	// Insert rows.
	_, err = m.Exec(ctx, `INSERT INTO test (name) VALUES (?), (?)`, "alice", "bob")
	if err != nil {
		t.Fatalf("Exec INSERT: %v", err)
	}

	// Query rows.
	rows, err := m.Query(ctx, `SELECT id, name FROM test ORDER BY id`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		names = append(names, name)
	}

	if len(names) != 2 || names[0] != "alice" || names[1] != "bob" {
		t.Fatalf("expected [alice, bob], got %v", names)
	}
}

func TestSQLiteStop(t *testing.T) {
	m, _ := setupSQLModule(t)
	ctx := context.Background()

	if err := m.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// After stop, DB should be closed.
	if err := m.Healthy(ctx); err == nil {
		t.Fatal("expected error after Stop")
	}
}

func TestSQLiteResourceProvided(t *testing.T) {
	_, k := setupSQLModule(t)

	// The module should provide both "data.sql" and "db" resources.
	_, ok := k.Resource("data.sql")
	if !ok {
		t.Fatal("expected data.sql resource")
	}

	_, ok = k.Resource("db")
	if !ok {
		t.Fatal("expected db resource")
	}
}
