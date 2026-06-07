---
title: Run SQL migrations from a module
weight: 12
---

Use this guide when a module owns its own database schema and needs to evolve
it over time.

## Solution

Register migrations on the `MigrationRegistry` during `Init`. The SQL module
runs them at `kernel.after_init`, in a transaction, before any module's
`Start`.

```go
import (
    "context"
    "database/sql"

    datasql "github.com/frob/nullspace/module/data/sql"
    "github.com/frob/nullspace/kernel"
)

func (m *Module) Init(k *kernel.Kernel) error {
    reg, err := kernel.GetResource[*datasql.MigrationRegistry](k, "data.sql.migrations")
    if err != nil {
        return err
    }

    reg.Register("widgets",
        datasql.Migration{
            Version:     1,
            Description: "create widgets",
            Up: func(ctx context.Context, tx *sql.Tx) error {
                _, err := tx.ExecContext(ctx, `
                    CREATE TABLE widgets (
                        id   INTEGER PRIMARY KEY AUTOINCREMENT,
                        name TEXT NOT NULL
                    )
                `)
                return err
            },
        },
        datasql.Migration{
            Version:     2,
            Description: "add created_at",
            Up: func(ctx context.Context, tx *sql.Tx) error {
                _, err := tx.ExecContext(ctx, `
                    ALTER TABLE widgets ADD COLUMN created_at TEXT
                        NOT NULL DEFAULT ''
                `)
                return err
            },
        },
    )

    return nil
}
```

## Rules

- **Versions are sequential integers, scoped per module.** Two modules can
  both have a version 1 — they don't collide.
- **Each migration runs in its own transaction.** A failed `Up` rolls back
  and stops subsequent migrations.
- **Migrations are forward-only.** Fix a bad migration by writing a new one.
- **Registration order is execution order.** Modules registered earlier with
  `k.Use()` run their migrations first.

## Tracking table

The SQL module creates `_migrations` automatically:

```sql
CREATE TABLE IF NOT EXISTS _migrations (
    module      TEXT    NOT NULL,
    version     INTEGER NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    applied_at  TEXT    NOT NULL,
    PRIMARY KEY (module, version)
);
```

Inspect what has run:

```sql
SELECT * FROM _migrations ORDER BY applied_at;
```

## Variations

### Conditional migrations

Branch on the SQL driver if you support more than one:

```go
Up: func(ctx context.Context, tx *sql.Tx) error {
    driver := datasql.DriverFromContext(ctx)
    if driver == "postgres" {
        _, err := tx.ExecContext(ctx, `CREATE TABLE ... (id BIGSERIAL PRIMARY KEY)`)
        return err
    }
    _, err := tx.ExecContext(ctx, `CREATE TABLE ... (id INTEGER PRIMARY KEY AUTOINCREMENT)`)
    return err
},
```

### Bundled framework migrations

Framework modules (e.g. `session` with the SQL store) register their own
migrations automatically. You don't need to create their tables yourself.

## See also

- [Use SQL storage with migrations]({{< relref "sql-data" >}})
