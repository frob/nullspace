// Package sql provides the SQL data module with pluggable database drivers.
//
// It exposes a standard *sql.DB for direct use by application modules.
// SQLite is the default driver (pure Go, no CGO) via modernc.org/sqlite.
package sql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/frob/nullspace/kernel"

	// Register the SQLite driver.
	_ "modernc.org/sqlite"
)

// Config holds the SQL module's configuration.
type Config struct {
	Driver string `json:"driver" toml:"driver"`
	DSN    string `json:"dsn" toml:"dsn"`
}

// Module provides SQL database access via database/sql.
type Module struct {
	db         *sql.DB
	kernel     *kernel.Kernel
	migrations *MigrationRegistry
}

// New creates a new SQL data module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "data.sql" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "data.sql",
		Default: Config{
			Driver: "sqlite",
			DSN:    "./data.db",
		},
		DefaultEnabled: false, // opt-in: not every app needs a database
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	var cfg Config
	if err := k.Config().Decode("data.sql", &cfg); err != nil {
		cfg = Config{Driver: "sqlite", DSN: "./data.db"}
	}

	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return fmt.Errorf("open database (%s): %w", cfg.Driver, err)
	}
	m.db = db

	m.migrations = NewMigrationRegistry()

	k.Provide("data.sql", m)
	k.Provide("db", db)
	k.Provide("data.sql.migrations", m.migrations)

	k.Hook("kernel.after_init", 10, func(ctx context.Context) error {
		return m.migrations.Run(ctx, m.db)
	})

	return nil
}

func (m *Module) Start(ctx context.Context) error {
	// Verify the connection works on start.
	return m.Healthy(ctx)
}

func (m *Module) Stop(ctx context.Context) error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Healthy checks the database connection.
func (m *Module) Healthy(ctx context.Context) error {
	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return m.db.PingContext(ctx)
}

// DB returns the underlying *sql.DB for direct use.
// Application modules retrieve this via the service locator:
//
//	db, err := kernel.GetResource[*sql.DB](k, "db")
func (m *Module) DB() *sql.DB {
	return m.db
}

// Query executes a query and fires data hooks.
func (m *Module) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := m.kernel.Fire("data.before_read", ctx); err != nil {
		return nil, err
	}

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	if err := m.kernel.Fire("data.after_read", ctx); err != nil {
		rows.Close()
		return nil, err
	}

	return rows, nil
}

// Exec executes a statement and fires data hooks.
func (m *Module) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if err := m.kernel.Fire("data.before_write", ctx); err != nil {
		return nil, err
	}

	result, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	if err := m.kernel.Fire("data.after_write", ctx); err != nil {
		return nil, err
	}

	return result, nil
}
