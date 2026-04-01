Modules
=======

Everything in Nullspace is a module. The kernel manages modules through a
consistent lifecycle, and modules communicate through the hook bus and service
locator.

Module Interface
----------------

Every module implements the ``Module`` interface:

.. code-block:: go

    type Module interface {
        Name() string
        Init(k *Kernel) error
        Start(ctx context.Context) error
        Stop(ctx context.Context) error
    }

- ``Name()`` returns a unique identifier (e.g., ``"data.static"``)
- ``Init()`` wires the module into the kernel: registers hooks, provides
  resources, reads configuration
- ``Start()`` begins runtime operation (e.g., start HTTP server)
- ``Stop()`` shuts down gracefully

Lifecycle
---------

Modules go through a strict lifecycle managed by the kernel:

::

    kernel.Use(module)         Register (stored in order)
           |
    kernel.Init(ctx)           For each enabled module:
           |                     1. Collect config defaults
           |                     2. Load TOML + env overrides
           |                     3. Check if enabled
           |                     4. Call module.Init(kernel)
           |
    kernel.Start(ctx)          For each enabled module:
           |                     Call module.Start(ctx) in registration order
           |
    kernel.Stop(ctx)           For each enabled module:
                                 Call module.Stop(ctx) in REVERSE order

.. important::

   Modules are initialized and started in **registration order** and stopped
   in **reverse order**. Register modules in dependency order -- if module B
   depends on a resource provided by module A, register A first.

Configuration
-------------

Modules that need configuration implement the ``Configurable`` interface:

.. code-block:: go

    type Configurable interface {
        Config() ModuleConfig
    }

    type ModuleConfig struct {
        Key            string  // TOML section path (e.g., "data.static")
        Default        any     // Default config struct
        DefaultEnabled bool    // Whether enabled without explicit config
    }

Example:

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

During Init, read the populated config:

.. code-block:: go

    func (m *Module) Init(k *kernel.Kernel) error {
        var cfg Config
        if err := k.Config().Decode("data.static", &cfg); err != nil {
            cfg = Config{Dir: "./public"} // fallback
        }
        m.dir = cfg.Dir
        return nil
    }

Enabling and Disabling
-----------------------

Modules can be enabled or disabled via the ``[modules]`` section in
``nullspace.toml``:

.. code-block:: toml

    [modules]
    "data.sql" = true       # enable the SQL module (disabled by default)
    "format.query_param" = false  # disable query param format resolution

Modules without an explicit entry use their ``DefaultEnabled`` value. Disabled
modules are not initialized, started, or stopped. Their hooks do not fire.

Service Locator
---------------

Modules share resources through the kernel's service locator:

.. code-block:: go

    // Provider (during Init)
    k.Provide("data.file", m)

    // Consumer (during Init of a later module)
    fileMod, err := kernel.GetResource[*file.Module](k, "data.file")

Standard resource keys provided by built-in modules:

============================  ========================
Key                           Type
============================  ========================
``"router"``                  ``*request.Router``
``"request.adapter"``         ``*request.Adapter``
``"response.pipeline"``       ``*response.Pipeline``
``"logger"``                  ``kernel.Logger``
``"data.static"``             ``*static.Module``
``"data.file"``               ``*file.Module``
``"data.sql"``                ``*sql.Module``
``"db"``                      ``*sql.DB``
``"session"``                 ``*session.Module``
``"session.store"``           ``session.Store``
``"http-security"``           ``*httpsecurity.Module``
``"websocket"``               ``*websocket.Module``
``"websocket.manager"``       ``*websocket.Manager``
============================  ========================

DataProvider
------------

Data-oriented modules can implement ``DataProvider``, which extends ``Module``
with a health check:

.. code-block:: go

    type DataProvider interface {
        Module
        Healthy(ctx context.Context) error
    }

The SQL, file, and static modules all implement ``DataProvider``.

Built-in Modules
-----------------

Nullspace ships with these modules:

=================================  ===============  ==============
Module Name                        Default Enabled  Package
=================================  ===============  ==============
``log``                            yes              ``nslog``
``request``                        yes              ``request``
``response``                       yes              ``response``
``format.route_override``          yes              ``response``
``format.query_param``             yes              ``response``
``format.content_negotiate``       yes              ``response``
``format.default``                 yes              ``response``
``data.static``                    yes              ``data/static``
``data.file``                      yes              ``data/file``
``data.sql``                       **no**           ``data/sql``
``session``                        **no**           ``session``
``http-security``                  **no**           ``httpsecurity``
``websocket``                      **no**           ``websocket``
=================================  ===============  ==============
