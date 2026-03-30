kernel
======

``import "github.com/frob/nullspace/kernel"``

The kernel package is the framework core. It provides the module registry,
hook bus, configuration primitive, service locator, and logger port.

Interfaces
----------

Module
~~~~~~

.. code-block:: go

    type Module interface {
        Name() string
        Init(k *Kernel) error
        Start(ctx context.Context) error
        Stop(ctx context.Context) error
    }

Base interface for all framework components.

Configurable
~~~~~~~~~~~~

.. code-block:: go

    type Configurable interface {
        Config() ModuleConfig
    }

Implemented by modules that declare a configuration section.

DataProvider
~~~~~~~~~~~~

.. code-block:: go

    type DataProvider interface {
        Module
        Healthy(ctx context.Context) error
    }

Extends Module with health checking for data modules.

Logger
~~~~~~

.. code-block:: go

    type Logger interface {
        Debug(msg string, args ...any)
        Info(msg string, args ...any)
        Warn(msg string, args ...any)
        Error(msg string, args ...any)
        With(args ...any) Logger
    }

Framework logging port. Arguments are key-value pairs.

Types
-----

ModuleConfig
~~~~~~~~~~~~

.. code-block:: go

    type ModuleConfig struct {
        Key            string
        Default        any
        DefaultEnabled bool
    }

Declares a module's TOML section key, default values, and default enabled state.

Kernel
~~~~~~

.. code-block:: go

    func New(opts ...Option) *Kernel

Creates a new kernel with the given options.

**Options:**

.. code-block:: go

    func WithLogger(l Logger) Option
    func WithConfigFile(path string) Option
    func WithEnvPrefix(prefix string) Option

**Lifecycle:**

.. code-block:: go

    func (k *Kernel) Use(m Module)
    func (k *Kernel) Init(ctx context.Context) error
    func (k *Kernel) Start(ctx context.Context) error
    func (k *Kernel) Stop(ctx context.Context) error

**Hooks:**

.. code-block:: go

    func (k *Kernel) Hook(name string, priority int, handler HookFunc)
    func (k *Kernel) HookResolve(name string, priority int, handler ResolveFunc)
    func (k *Kernel) Fire(name string, ctx context.Context) error
    func (k *Kernel) Resolve(name string, ctx context.Context) (any, error)
    func (k *Kernel) HookBus() *HookBus

**Service locator:**

.. code-block:: go

    func (k *Kernel) Provide(key string, value any)
    func (k *Kernel) Resource(key string) (any, bool)
    func GetResource[T any](k *Kernel, key string) (T, error)

**Logger:**

.. code-block:: go

    func (k *Kernel) Logger() Logger
    func (k *Kernel) SetLogger(l Logger)

**Config:**

.. code-block:: go

    func (k *Kernel) Config() *Config

HookFunc
~~~~~~~~

.. code-block:: go

    type HookFunc func(ctx context.Context) error

Standard hook handler.

ResolveFunc
~~~~~~~~~~~

.. code-block:: go

    type ResolveFunc func(ctx context.Context) (any, bool, error)

Resolution hook handler. Returns ``(value, resolved, error)``. When
``resolved`` is true, the chain stops.

HookBus
~~~~~~~

.. code-block:: go

    func NewHookBus() *HookBus
    func (b *HookBus) On(name, module string, priority int, handler HookFunc)
    func (b *HookBus) OnResolve(name, module string, priority int, handler ResolveFunc)
    func (b *HookBus) Fire(name string, ctx context.Context) error
    func (b *HookBus) Resolve(name string, ctx context.Context) (any, error)
    func (b *HookBus) Hooks() []string

Config
~~~~~~

.. code-block:: go

    func NewConfig() *Config
    func (c *Config) SetModuleEnabled(name string, enabled bool)
    func (c *Config) ModuleEnabled(name string) bool
    func (c *Config) Set(key string, value any)
    func (c *Config) Get(key string) (any, bool)
    func (c *Config) Decode(key string, target any) error
    func (c *Config) Snapshot() *Snapshot

Snapshot
~~~~~~~~

.. code-block:: go

    func (s *Snapshot) ModuleEnabled(name string) bool
    func (s *Snapshot) Get(key string) (any, bool)
    func (s *Snapshot) Decode(key string, target any) error

Context Helpers
~~~~~~~~~~~~~~~

.. code-block:: go

    func SnapshotFromContext(ctx context.Context) *Snapshot
    func ContextWithSnapshot(ctx context.Context, snap *Snapshot) context.Context

Logger Constructors
~~~~~~~~~~~~~~~~~~~

.. code-block:: go

    func NewSlogLogger() Logger
    func NewSlogLoggerFrom(l *slog.Logger) Logger
