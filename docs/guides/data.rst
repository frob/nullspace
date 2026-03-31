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
