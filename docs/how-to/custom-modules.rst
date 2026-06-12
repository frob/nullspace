Writing Custom Modules
======================

Modules are the primary extension point in Nullspace. This guide covers
creating modules from simple to advanced.

Basic Module
------------

A minimal module implements the ``Module`` interface:

.. code-block:: go

    package mymodule

    import (
        "context"
        "github.com/frob/nullspace/kernel"
    )

    type Module struct{}

    func New() *Module { return &Module{} }

    func (m *Module) Name() string                       { return "mymodule" }
    func (m *Module) Init(k *kernel.Kernel) error        { return nil }
    func (m *Module) Start(ctx context.Context) error    { return nil }
    func (m *Module) Stop(ctx context.Context) error     { return nil }

Register it:

.. code-block:: go

    k.Use(mymodule.New())

Configurable Module
-------------------

Add configuration by implementing ``Configurable``:

.. code-block:: go

    type Config struct {
        APIKey  string `json:"api_key"`
        Timeout int    `json:"timeout"`
    }

    type Module struct {
        config Config
    }

    func (m *Module) Config() kernel.ModuleConfig {
        return kernel.ModuleConfig{
            Key: "mymodule",
            Default: Config{
                Timeout: 30,
            },
            DefaultEnabled: true,
        }
    }

    func (m *Module) Init(k *kernel.Kernel) error {
        if err := k.Config().Decode("mymodule", &m.config); err != nil {
            m.config = Config{Timeout: 30}
        }
        return nil
    }

TOML configuration:

.. code-block:: toml

    [mymodule]
    api_key = "secret"
    timeout = 60

Environment override::

    NULLSPACE_MYMODULE_API_KEY=secret

Module with Hooks
-----------------

Register hooks during ``Init()`` to participate in lifecycle events:

.. code-block:: go

    type MetricsModule struct {
        kernel *kernel.Kernel
    }

    func (m *MetricsModule) Init(k *kernel.Kernel) error {
        m.kernel = k

        // Log every request's duration
        k.Hook("request.complete", 80, m.recordDuration)

        // Log data access patterns
        k.Hook("data.before_read", 50, m.recordRead)

        return nil
    }

    func (m *MetricsModule) recordDuration(ctx context.Context) error {
        // Extract timing info from context
        // Record to your metrics backend
        return nil
    }

    func (m *MetricsModule) recordRead(ctx context.Context) error {
        // Track which data is being accessed
        return nil
    }

Module with Routes (TOML)
--------------------------

The preferred way to define routes is via an embedded ``routes.toml``:

.. code-block:: go

    //go:embed routes.toml
    var routesData []byte

    func (m *Module) Init(k *kernel.Kernel) error {
        // Register named handlers.
        reg, _ := kernel.GetResource[*routing.Registry](k, "routing.registry")
        reg.HandleFunc("myresource.list", m.list)
        reg.HandleFunc("myresource.get", m.get)

        // Load this module's route file.
        routingMod, _ := kernel.GetResource[*routing.Module](k, "routing")
        routingMod.LoadRoutes(routesData)
        return nil
    }

.. code-block:: toml

    # routes.toml
    [[routes]]
    path = "/api/myresource"
    handler = "myresource.list"
    format = "json"

    [[routes]]
    path = "/api/myresource/:id"
    handler = "myresource.get"
    format = "json"

This pattern keeps routes co-located with the module code. The routes are
compiled into the binary via ``go:embed`` and merged at startup.

Module with Routes (Go)
------------------------

For routes that need to be defined programmatically, use the router directly:

.. code-block:: go

    func (m *APIModule) Init(k *kernel.Kernel) error {
        router, _ := kernel.GetResource[*request.Router](k, "router")

        router.Get("/api/myresource", m.list)
        router.Get("/api/myresource/:id", m.get)
        router.Post("/api/myresource", m.create)

        return nil
    }

.. important::

   The routing module must be registered before modules that call
   ``LoadRoutes()``. The request adapter must be registered before
   modules that access the router directly.

Module with Resources
---------------------

Provide resources for other modules to consume:

.. code-block:: go

    func (m *CacheModule) Init(k *kernel.Kernel) error {
        m.cache = NewRedisCache(m.config.URL)
        k.Provide("cache", m.cache)
        return nil
    }

Other modules consume it:

.. code-block:: go

    func (m *APIModule) Init(k *kernel.Kernel) error {
        cache, err := kernel.GetResource[*Cache](k, "cache")
        if err != nil {
            return err
        }
        m.cache = cache
        return nil
    }

Module with Middleware
~~~~~~~~~~~~~~~~~~~~~~

Modules can register middleware on the request adapter:

.. code-block:: go

    func (m *AuthModule) Init(k *kernel.Kernel) error {
        adapter, err := kernel.GetResource[*request.Adapter](k, "request.adapter")
        if err != nil {
            return err
        }
        adapter.Use(m.authenticate)
        return nil
    }

    func (m *AuthModule) authenticate(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            token := ctx.Request.Header.Get("Authorization")
            if !m.validate(token) {
                ctx.Writer.WriteHeader(http.StatusUnauthorized)
                return fmt.Errorf("unauthorized")
            }
            return next(ctx)
        }
    }

Data Provider Module
--------------------

Data modules should implement ``DataProvider`` for health checking:

.. code-block:: go

    type RedisModule struct {
        client *redis.Client
        kernel *kernel.Kernel
    }

    func (m *RedisModule) Healthy(ctx context.Context) error {
        return m.client.Ping(ctx).Err()
    }

    func (m *RedisModule) Get(ctx context.Context, key string) (string, error) {
        // Fire data hooks
        if err := m.kernel.Fire("data.before_read", ctx); err != nil {
            return "", err
        }

        val, err := m.client.Get(ctx, key).Result()
        if err != nil {
            return "", err
        }

        if err := m.kernel.Fire("data.after_read", ctx); err != nil {
            return "", err
        }

        return val, nil
    }

Format Resolver Module
-----------------------

Create a custom format resolver by registering a resolution hook:

.. code-block:: go

    type FormatFromAPIKey struct{}

    func (m *FormatFromAPIKey) Name() string { return "format.api_key" }

    func (m *FormatFromAPIKey) Config() kernel.ModuleConfig {
        return kernel.ModuleConfig{
            Key:            "format.api_key",
            DefaultEnabled: true,
        }
    }

    func (m *FormatFromAPIKey) Init(k *kernel.Kernel) error {
        // Priority 15: after route override (10), before query param (20)
        k.HookResolve("response.format.resolve", 15, m.resolve)
        return nil
    }

    func (m *FormatFromAPIKey) resolve(ctx context.Context) (any, bool, error) {
        // Look up API key tier to determine format
        apiKey := getAPIKeyFromContext(ctx)
        if apiKey != "" && isPremiumKey(apiKey) {
            return "json", true, nil  // premium users always get JSON
        }
        return nil, false, nil  // not resolved, try next
    }

Module Registration Order
-------------------------

Modules are initialized in registration order. Follow this pattern:

.. code-block:: go

    // 1. Logging (needed by everything)
    k.Use(nslog.New())

    // 2. Request adapter (provides router)
    k.Use(request.NewAdapter())

    // 3. Response pipeline (provides formatters)
    k.Use(response.NewPipeline())

    // 4. Format resolvers
    k.Use(response.NewFormatRouteOverride())
    k.Use(response.NewFormatQueryParam())
    k.Use(response.NewFormatDefault())

    // 5. Data modules
    k.Use(static.New())
    k.Use(file.New())
    k.Use(dbmod.New())

    // 6. Routing module (provides registry, resolves routes after all inits)
    k.Use(routing.New())

    // 7. Application modules (register handlers/middleware on routing registry)
    k.Use(mymodule.New())

.. seealso::

   **Examples**

   The kitchen-sink example includes three custom modules that demonstrate the patterns above:

   - ``cmd/examples/kitchen-sink/modules/auth/module.go`` -- middleware as a module, config-driven path protection, per-module route file
   - ``cmd/examples/kitchen-sink/modules/forms/module.go`` -- declarative TOML forms, data layer integration, custom hook points
   - ``cmd/examples/kitchen-sink/modules/chat/module.go`` -- WebSocket handler registration, room-based broadcast, middleware state injection
   - ``cmd/examples/kitchen-sink/main.go`` -- module registration order and custom handler wiring

   See :doc:`/explanation/example-modules` for a detailed walkthrough of these modules.

   **Source code**

   - Module and Configurable interfaces: ``kernel/kernel.go``
   - Service locator (Provide / GetResource): ``kernel/kernel.go``
