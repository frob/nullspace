Data Layer
==========

The data layer provides pluggable data storage through the ``DataProvider``
port. Nullspace ships with three data modules: static files, file-based
entities, and SQL.

DataProvider Port
-----------------

All data modules implement ``DataProvider``:

.. code-block:: go

    type DataProvider interface {
        Module
        Healthy(ctx context.Context) error
    }

The kernel manages provider lifecycle. ``Healthy()`` is available for health
check endpoints.

Policy Hooks
------------

Data modules fire hooks that policy modules can intercept:

- ``data.before_read`` -- Fires before any data read
- ``data.after_read`` -- Fires after a successful read
- ``data.before_write`` -- Fires before any data write
- ``data.after_write`` -- Fires after a successful write

Example policy module:

.. code-block:: go

    func (p *PolicyModule) Init(k *kernel.Kernel) error {
        k.Hook("data.before_read", 10, p.checkReadAccess)
        k.Hook("data.before_write", 10, p.checkWriteAccess)
        return nil
    }

    func (p *PolicyModule) checkReadAccess(ctx context.Context) error {
        // Return an error to deny access
        return nil
    }

Static Files
------------

Serves files from a directory as a request fallback.

**Config:**

.. code-block:: toml

    [data.static]
    dir = "./public"

**Usage:**

.. code-block:: go

    import "github.com/frob/nullspace/module/data/static"

    k.Use(static.New())

**Behavior:**

- Serves files from the configured directory
- Only runs when no dynamic route matches (fallback)
- Sets ``Content-Type`` from file extension
- Serves ``index.html`` for directory paths (``/docs/`` serves ``docs/index.html``)
- Prevents directory traversal attacks
- No directory listing

.. note::

   Register the static module **after** the request adapter so it can
   register itself as a fallback handler.

File-Based Entities
-------------------

Stores records as files on disk. Directory = collection, filename = entity ID.

**Config:**

.. code-block:: toml

    [data.file]
    dir = "./content"
    format = "markdown"

**Usage:**

.. code-block:: go

    import "github.com/frob/nullspace/module/data/file"

    fileMod := file.New()
    k.Use(fileMod)

**CRUD Operations:**

.. code-block:: go

    ctx := context.Background()

    // Read
    entity, err := fileMod.Read(ctx, "posts", "hello-world")

    // List
    entities, err := fileMod.List(ctx, "posts")

    // Write
    entity := &file.Entity{
        ID:     "new-post",
        Meta:   map[string]any{"title": "New Post"},
        Body:   "Content here.",
        Format: "markdown",
    }
    err := fileMod.Write(ctx, "posts", "new-post", entity)

    // Delete
    err := fileMod.Delete(ctx, "posts", "old-post")

Entity Type
~~~~~~~~~~~

.. code-block:: go

    type Entity struct {
        ID     string         // From filename (without extension)
        Meta   map[string]any // Structured fields
        Body   string         // Content body
        Format string         // "markdown", "json", "toml"
    }

Supported Formats
~~~~~~~~~~~~~~~~~

**Markdown with YAML frontmatter:**

.. code-block:: markdown

    ---
    title: My Post
    date: 2025-01-15
    tags:
      - go
      - web
    ---

    The body content goes here.

**Markdown with TOML frontmatter:**

.. code-block:: markdown

    +++
    title = "My Post"
    date = 2025-01-15
    tags = ["go", "web"]
    +++

    The body content goes here.

**JSON:**

.. code-block:: json

    {
        "title": "My Post",
        "body": "Content here.",
        "tags": ["go", "web"]
    }

The ``body`` key is extracted as the entity body; all other keys go into
``Meta``.

**TOML:**

.. code-block:: toml

    title = "My Post"
    body = "Content here."
    tags = ["go", "web"]

Directory Structure
~~~~~~~~~~~~~~~~~~~

::

    content/
    ├── posts/
    │   ├── hello-world.md
    │   └── architecture.md
    ├── pages/
    │   └── about.md
    └── authors/
        └── jane.toml

``Read(ctx, "posts", "hello-world")`` reads ``content/posts/hello-world.md``.
The module tries extensions ``.md``, ``.markdown``, ``.json``, ``.toml`` when
looking up files.

SQL
---

Provides SQL database access via ``database/sql`` with SQLite as the default
driver.

**Config:**

.. code-block:: toml

    [modules]
    "data.sql" = true    # opt-in: disabled by default

    [data.sql]
    driver = "sqlite"
    dsn = "./data.db"

**Usage:**

.. code-block:: go

    import dbmod "github.com/frob/nullspace/module/data/sql"

    k.Use(dbmod.New())

**Querying:**

.. code-block:: go

    // Via the module (fires data hooks)
    rows, err := sqlMod.Query(ctx, "SELECT id, name FROM users WHERE active = ?", true)
    result, err := sqlMod.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "alice")

    // Via raw *sql.DB (bypasses hooks)
    db := sqlMod.DB()
    rows, err := db.QueryContext(ctx, "SELECT ...")

**From other modules (via service locator):**

.. code-block:: go

    func (m *UserModule) Init(k *kernel.Kernel) error {
        db, err := kernel.GetResource[*sql.DB](k, "db")
        if err != nil {
            return err
        }
        m.db = db
        return nil
    }

**Drivers:**

SQLite is included via ``modernc.org/sqlite`` (pure Go, no CGO). For other
databases, import the driver and set the config:

.. code-block:: toml

    [data.sql]
    driver = "postgres"
    dsn = "postgres://user:pass@localhost/mydb"

.. code-block:: go

    import _ "github.com/lib/pq"  // PostgreSQL driver

**Health checks:**

.. code-block:: go

    err := sqlMod.Healthy(ctx)  // calls db.PingContext()

Migrations
----------

The SQL module includes a built-in migration system for managing schema changes.
Migrations are Go functions that run inside transactions — if one fails, its
transaction is rolled back and the error is reported.

How It Works
~~~~~~~~~~~~

1. The SQL module creates a ``MigrationRegistry`` during ``Init`` and provides
   it via the service locator as ``"data.sql.migrations"``.
2. Other modules register their migrations during their own ``Init``.
3. At ``kernel.after_init``, the SQL module runs all pending migrations
   automatically — before any module's ``Start`` method is called.

Each migration is scoped to a module name and version number. The tracking table
``_migrations`` records which migrations have been applied, so they run exactly
once.

Defining Migrations
~~~~~~~~~~~~~~~~~~~

Migrations are registered from a module's ``Init`` method:

.. code-block:: go

    import (
        datasql "github.com/frob/nullspace/module/data/sql"
        "github.com/frob/nullspace/kernel"
    )

    func (m *MyModule) Init(k *kernel.Kernel) error {
        reg, err := kernel.GetResource[*datasql.MigrationRegistry](k, "data.sql.migrations")
        if err != nil {
            return err
        }

        reg.Register("mymodule",
            datasql.Migration{
                Version:     1,
                Description: "create widgets table",
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
                Description: "add created_at to widgets",
                Up: func(ctx context.Context, tx *sql.Tx) error {
                    _, err := tx.ExecContext(ctx, `
                        ALTER TABLE widgets ADD COLUMN created_at TEXT NOT NULL DEFAULT ''
                    `)
                    return err
                },
            },
        )

        return nil
    }

Key points:

- **Versions are sequential integers, scoped per module.** The session module
  has its own version 1, your app has its own version 1 — they do not collide.
- **Each migration runs in a transaction.** If ``Up`` returns an error, the
  transaction is rolled back and subsequent migrations do not run.
- **Migrations are forward-only.** There is no rollback support. To fix a bad
  migration, write a new one.
- **Registration order determines module execution order.** Modules registered
  earlier with ``k.Use()`` have their migrations run first. Within a module,
  migrations run in ascending version order.

The ``_migrations`` tracking table is created automatically:

.. code-block:: sql

    CREATE TABLE IF NOT EXISTS _migrations (
        module      TEXT    NOT NULL,
        version     INTEGER NOT NULL,
        description TEXT    NOT NULL DEFAULT '',
        applied_at  TEXT    NOT NULL,
        PRIMARY KEY (module, version)
    );

Framework modules like ``session`` register their own migrations automatically
when using the SQL store — you do not need to create the ``sessions`` table
yourself.
