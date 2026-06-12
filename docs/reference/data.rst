data
====

The data packages provide pluggable data storage modules.

data/static
-----------

``import "github.com/frob/nullspace/module/data/static"``

Serves static files from a directory as a request fallback.

.. code-block:: go

    func New() *Module

Creates a new static file module. Implements ``Module``, ``Configurable``,
and ``DataProvider``.

**Config struct:**

.. code-block:: go

    type Config struct {
        Dir string `json:"dir"`  // default "./public"
    }

data/file
---------

``import "github.com/frob/nullspace/module/data/file"``

File-based entity storage. One file per record, directory per collection.

Entity
~~~~~~

.. code-block:: go

    type Entity struct {
        ID     string         // From filename (without extension)
        Meta   map[string]any // Structured fields (frontmatter or document)
        Body   string         // Content body
        Format string         // "markdown", "json", "toml"
    }

Module
~~~~~~

.. code-block:: go

    func New() *Module

Creates a new file data module. Implements ``Module``, ``Configurable``,
and ``DataProvider``.

.. code-block:: go

    func (m *Module) Read(ctx context.Context, collection, id string) (*Entity, error)
    func (m *Module) List(ctx context.Context, collection string) ([]*Entity, error)
    func (m *Module) Write(ctx context.Context, collection, id string, entity *Entity) error
    func (m *Module) Delete(ctx context.Context, collection, id string) error
    func (m *Module) Healthy(ctx context.Context) error

All CRUD operations fire ``data.before_read``/``data.after_read`` or
``data.before_write``/``data.after_write`` hooks.

**Config struct:**

.. code-block:: go

    type Config struct {
        Dir    string `json:"dir"`    // default "./content"
        Format string `json:"format"` // default "markdown"
    }

**Supported file formats:**

- ``.md``, ``.markdown`` -- Markdown with YAML (``---``) or TOML (``+++``) frontmatter
- ``.json`` -- JSON (``body`` key extracted as entity body)
- ``.toml`` -- TOML (``body`` key extracted as entity body)

data/sql
--------

``import "github.com/frob/nullspace/module/data/sql"``

SQL database access via ``database/sql`` with SQLite default.

.. code-block:: go

    func New() *Module

Creates a new SQL module. Implements ``Module``, ``Configurable``, and
``DataProvider``. **Disabled by default** -- enable via config.

.. code-block:: go

    func (m *Module) DB() *sql.DB
    func (m *Module) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    func (m *Module) Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
    func (m *Module) Healthy(ctx context.Context) error

``Query`` and ``Exec`` fire data hooks. ``DB()`` returns the raw
``*sql.DB`` for direct use (bypasses hooks).

**Config struct:**

.. code-block:: go

    type Config struct {
        Driver string `json:"driver"` // default "sqlite"
        DSN    string `json:"dsn"`    // default "./data.db"
    }

**Service locator keys:**

- ``"data.sql"`` -- ``*sql.Module``
- ``"db"`` -- ``*sql.DB``
- ``"data.sql.migrations"`` -- ``*sql.MigrationRegistry``

Migrations
~~~~~~~~~~

.. code-block:: go

    type Migration struct {
        Version     int
        Description string
        Up          func(ctx context.Context, tx *sql.Tx) error
    }

A single, versioned schema change. ``Up`` runs inside a transaction — return
an error to roll back.

.. code-block:: go

    func NewMigrationRegistry() *MigrationRegistry

Creates an empty migration registry. The SQL module creates one during
``Init`` and provides it as ``"data.sql.migrations"``.

.. code-block:: go

    func (r *MigrationRegistry) Register(module string, migrations ...Migration)

Registers migrations for the named module. Call this during ``Init`` from any
module that needs database tables. Modules are processed in registration order;
within a module, migrations run in ascending version order.

.. code-block:: go

    func (r *MigrationRegistry) Run(ctx context.Context, db *sql.DB) error

Creates the ``_migrations`` tracking table if needed, then applies all pending
migrations. Each migration runs in its own transaction. Called automatically by
the SQL module at ``kernel.after_init`` — you do not need to call this directly.
