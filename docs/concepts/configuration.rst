Configuration
=============

Configuration is a kernel primitive, not a module. It loads before any module
initializes and provides per-request immutable snapshots.

Sources and Precedence
----------------------

Configuration values are resolved in this order (highest priority wins):

::

    1. Module defaults (compiled in)     lowest priority
    2. TOML config file                  overrides defaults
    3. Environment variables             overrides TOML

TOML Config File
----------------

By default, the kernel looks for ``nullspace.toml`` in the working directory.
Override with ``kernel.WithConfigFile()``:

.. code-block:: go

    k := kernel.New(
        kernel.WithConfigFile("/etc/myapp/config.toml"),
    )

Full example:

.. code-block:: toml

    [request]
    addr = ":8080"

    [log]
    level = "info"
    format = "text"

    [response]
    default_format = "json"
    template_dir = "./templates"

    [data.static]
    dir = "./public"

    [data.file]
    dir = "./content"
    format = "markdown"

    [data.sql]
    driver = "sqlite"
    dsn = "./data.db"

    [modules]
    "data.sql" = true
    "format.query_param" = false

No config file is required. The framework works with zero configuration using
module defaults.

Environment Variables
---------------------

Environment variables override TOML values using a prefix + path convention::

    NULLSPACE_<SECTION>_<KEY>=value

Underscores in the variable name become dots in the config path:

========================================  ============================
Environment Variable                      Config Path
========================================  ============================
``NULLSPACE_LOG_LEVEL``                   ``log.level``
``NULLSPACE_REQUEST_ADDR``                ``request.addr``
``NULLSPACE_DATA_STATIC_DIR``             ``data.static.dir``
``NULLSPACE_DATA_SQL_DSN``                ``data.sql.dsn``
========================================  ============================

The prefix is configurable:

.. code-block:: go

    k := kernel.New(
        kernel.WithEnvPrefix("MYAPP"),
    )

    // Now uses MYAPP_LOG_LEVEL instead of NULLSPACE_LOG_LEVEL

Module-Declared Config
-----------------------

Each module declares its config section, key, defaults, and default enabled
state via the ``Configurable`` interface:

.. code-block:: go

    func (m *Module) Config() kernel.ModuleConfig {
        return kernel.ModuleConfig{
            Key: "data.static",
            Default: Config{
                Dir: "./public",
            },
            DefaultEnabled: true,
        }
    }

The kernel collects all declarations, loads the TOML file, applies env
overrides, and merges the result with defaults. Unspecified TOML keys retain
their default values.

Reading Config in Modules
--------------------------

During ``Init()``, call ``Decode()`` to read the populated config into a
typed struct:

.. code-block:: go

    func (m *Module) Init(k *kernel.Kernel) error {
        var cfg Config
        if err := k.Config().Decode("data.static", &cfg); err != nil {
            // handle error or use defaults
        }
        m.dir = cfg.Dir
        return nil
    }

Config structs use ``json`` tags for field mapping:

.. code-block:: go

    type Config struct {
        Dir    string `json:"dir"`
        Format string `json:"format"`
    }

Per-Request Snapshots
---------------------

The kernel holds a "live" config that can be mutated at runtime. When a
request starts, the live config is snapshotted:

.. code-block:: go

    snap := k.Config().Snapshot()
    ctx = kernel.ContextWithSnapshot(ctx, snap)

The snapshot is:

- **Immutable** -- Cannot be modified after creation
- **Attached to context** -- Available throughout the request lifecycle
- **Isolated** -- Changes to live config do not affect in-flight requests

This means a request always completes with the same configuration it started
with.

Accessing the snapshot in handlers:

.. code-block:: go

    func myHandler(ctx *request.Context) error {
        snap := ctx.Snapshot()
        val, ok := snap.Get("custom.key")
        // ...
    }

Module Enabled/Disabled
-----------------------

The ``[modules]`` section controls which modules are active:

.. code-block:: toml

    [modules]
    "data.sql" = true
    "format.query_param" = false

When a module is disabled:

- Its ``Init()``, ``Start()``, ``Stop()`` are not called
- Its hooks do not fire (checked per-request via config snapshot)
- It does not consume resources

Runtime Config Changes
-----------------------

The live config can be mutated at runtime:

.. code-block:: go

    k.Config().Set("custom.key", "new_value")
    k.Config().SetModuleEnabled("format.query_param", false)

Changes take effect at the **next request boundary**, not mid-request.
In-flight requests continue using their snapshot.
