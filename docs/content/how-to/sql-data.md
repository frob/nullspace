---
title: Use SQL storage with migrations
weight: 5
---

Use this guide when you need a real database instead of files on disk.

## Solution

Enable the `data.sql` module (it is opt-in) and point it at a database. SQLite
ships built in — no CGO required.

```toml
[modules]
"data.sql" = true

[data.sql]
driver = "sqlite"
dsn    = "./app.db"
```

Register the module after the request and response modules:

```go
import dbmod "github.com/frob/nullspace/module/data/sql"

k.Use(dbmod.New())
```

Query through the module so reads and writes fire the `data.before_*` /
`data.after_*` hooks:

```go
sqlMod, _ := kernel.GetResource[*dbmod.Module](k, "data.sql")

rows, err := sqlMod.Query(ctx,
    "SELECT id, name FROM users WHERE active = ?", true)
result, err := sqlMod.Exec(ctx,
    "INSERT INTO users (name) VALUES (?)", "alice")
```

Or grab the raw `*sql.DB` via the service locator (bypasses hooks):

```go
db, err := kernel.GetResource[*sql.DB](k, "db")
rows, err := db.QueryContext(ctx, "...")
```

## Add schema migrations

The SQL module runs migrations at `kernel.after_init`, before any module's
`Start`. Register migrations from your module's `Init`:

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

    reg.Register("myapp",
        datasql.Migration{
            Version:     1,
            Description: "create users",
            Up: func(ctx context.Context, tx *sql.Tx) error {
                _, err := tx.ExecContext(ctx, `
                    CREATE TABLE users (
                        id   INTEGER PRIMARY KEY AUTOINCREMENT,
                        name TEXT NOT NULL
                    )
                `)
                return err
            },
        },
    )
    return nil
}
```

Migrations are forward-only and scoped per module. Each runs once in its own
transaction.

## Variations

### Use a different driver

Import the driver blank and set the config:

```go
import _ "github.com/lib/pq"
```

```toml
[data.sql]
driver = "postgres"
dsn    = "postgres://user:pass@localhost/myapp"
```

### Set the DSN from the environment

```bash
NULLSPACE_DATA_SQL_DSN=postgres://user:pass@db:5432/myapp
```

### Health check

```go
if err := sqlMod.Healthy(ctx); err != nil {
    // db.PingContext() failed
}
```

## See also

- [Run SQL migrations from a module]({{< relref "migrations" >}})
- [Generate CRUD routes for a collection]({{< relref "collections" >}})
