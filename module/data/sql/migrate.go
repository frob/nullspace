package sql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Migration represents a single, versioned schema change owned by a module.
// Each migration runs inside a transaction — if Up returns an error the
// transaction is rolled back and no record is written to _migrations.
type Migration struct {
	Version     int
	Description string
	Up          func(ctx context.Context, tx *sql.Tx) error
}

// MigrationRegistry collects migrations from multiple modules and applies
// them in registration order (module) then version order (within a module).
type MigrationRegistry struct {
	mu         sync.Mutex
	migrations map[string][]Migration
	order      []string // insertion-ordered module names
}

// NewMigrationRegistry creates an empty registry.
func NewMigrationRegistry() *MigrationRegistry {
	return &MigrationRegistry{
		migrations: make(map[string][]Migration),
	}
}

// Register adds one or more migrations for the named module.
// Modules call this during Init to declare their schema changes.
func (r *MigrationRegistry) Register(module string, migrations ...Migration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.migrations[module]; !exists {
		r.order = append(r.order, module)
	}
	r.migrations[module] = append(r.migrations[module], migrations...)
}

// Run creates the _migrations tracking table (if needed) and applies every
// pending migration. Migrations are processed in module-registration order,
// and within each module in ascending version order.
func (r *MigrationRegistry) Run(ctx context.Context, db *sql.DB) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}

	applied, err := loadApplied(ctx, db)
	if err != nil {
		return err
	}

	for _, module := range r.order {
		migrations := r.migrations[module]

		// Sort by version within the module.
		sort.Slice(migrations, func(i, j int) bool {
			return migrations[i].Version < migrations[j].Version
		})

		for _, m := range migrations {
			if applied[migrationKey{module, m.Version}] {
				continue
			}
			if err := applyMigration(ctx, db, module, m); err != nil {
				return fmt.Errorf("migration %s v%d (%s): %w", module, m.Version, m.Description, err)
			}
		}
	}

	return nil
}

type migrationKey struct {
	module  string
	version int
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			module      TEXT    NOT NULL,
			version     INTEGER NOT NULL,
			description TEXT    NOT NULL DEFAULT '',
			applied_at  TEXT    NOT NULL,
			PRIMARY KEY (module, version)
		)
	`)
	return err
}

func loadApplied(ctx context.Context, db *sql.DB) (map[migrationKey]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT module, version FROM _migrations`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[migrationKey]bool)
	for rows.Next() {
		var k migrationKey
		if err := rows.Scan(&k.module, &k.version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[k] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, db *sql.DB, module string, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	if err := m.Up(ctx, tx); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO _migrations (module, version, description, applied_at) VALUES (?, ?, ?, ?)`,
		module, m.Version, m.Description, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}
