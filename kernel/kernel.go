package kernel

import (
	"context"
	"fmt"
	"sync"
)

// Option configures a Kernel during construction.
type Option func(*Kernel)

// WithLogger sets the kernel's logger, replacing the default slog-based logger.
func WithLogger(l Logger) Option {
	return func(k *Kernel) { k.logger = l }
}

// WithConfigFile sets the path to the TOML configuration file.
func WithConfigFile(path string) Option {
	return func(k *Kernel) { k.configFile = path }
}

// WithEnvPrefix sets the environment variable prefix for config overrides.
// Defaults to "NULLSPACE".
func WithEnvPrefix(prefix string) Option {
	return func(k *Kernel) { k.envPrefix = prefix }
}

// Kernel is the central registry and lifecycle manager for the framework.
// It manages modules, the hook bus, configuration, and the service locator.
type Kernel struct {
	modules    []Module
	resources  map[string]any
	hooks      *HookBus
	config     *Config
	logger     Logger
	configFile string
	envPrefix  string

	// initModule tracks which module is currently being initialized,
	// allowing Hook/HookResolve to auto-associate the owning module.
	initModule string

	mu sync.RWMutex
}

// New creates a new Kernel with the given options.
func New(opts ...Option) *Kernel {
	k := &Kernel{
		resources:  make(map[string]any),
		hooks:      NewHookBus(),
		config:     NewConfig(),
		envPrefix:  "NULLSPACE",
		configFile: "nullspace.toml",
	}

	for _, opt := range opts {
		opt(k)
	}

	// Default logger if none provided.
	if k.logger == nil {
		k.logger = NewSlogLogger()
	}

	return k
}

// Use registers a module with the kernel. Modules are initialized and started
// in registration order, and stopped in reverse order.
func (k *Kernel) Use(m Module) {
	k.modules = append(k.modules, m)
}

// Init initializes the kernel and all enabled modules.
//
// Lifecycle:
//  1. Collect module config declarations and apply defaults
//  2. Load external config (TOML file + env overrides)
//  3. Fire kernel.before_init hooks
//  4. Init each enabled module (in registration order)
//  5. Fire kernel.after_init hooks
func (k *Kernel) Init(ctx context.Context) error {
	// Step 1: Collect module config defaults and enabled states.
	for _, m := range k.modules {
		if c, ok := m.(Configurable); ok {
			mc := c.Config()
			k.config.SetModuleEnabled(m.Name(), mc.DefaultEnabled)
			if mc.Default != nil {
				k.config.Set(mc.Key, mc.Default)
			}
		} else {
			// Modules without config default to enabled.
			k.config.SetModuleEnabled(m.Name(), true)
		}
	}

	// Step 2: Load external config (TOML file + env overrides).
	raw, err := loadTOMLFile(k.configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	applyEnvOverrides(raw, k.envPrefix)

	// Apply [modules] enabled/disabled from config file.
	applyModulesSection(raw, k.config)

	// Merge loaded config sections with module defaults.
	if err := applyModuleConfigs(raw, k.modules, k.config); err != nil {
		return fmt.Errorf("apply config: %w", err)
	}

	k.logger.Debug("config loaded", "file", k.configFile)

	// Step 3: Fire kernel.before_init.
	if err := k.hooks.Fire("kernel.before_init", ctx); err != nil {
		return fmt.Errorf("kernel.before_init: %w", err)
	}

	// Step 4: Init enabled modules.
	for _, m := range k.modules {
		if !k.config.ModuleEnabled(m.Name()) {
			k.logger.Info("module disabled, skipping init", "module", m.Name())
			continue
		}

		k.logger.Info("initializing module", "module", m.Name())
		k.initModule = m.Name()
		if err := m.Init(k); err != nil {
			k.initModule = ""
			return fmt.Errorf("init %s: %w", m.Name(), err)
		}
		k.initModule = ""
	}

	// Step 5: Fire kernel.after_init.
	if err := k.hooks.Fire("kernel.after_init", ctx); err != nil {
		return fmt.Errorf("kernel.after_init: %w", err)
	}

	return nil
}

// Start starts all enabled modules.
//
// Lifecycle:
//  1. Fire kernel.before_start hooks
//  2. Start each enabled module (in registration order)
//  3. Fire kernel.after_start hooks
func (k *Kernel) Start(ctx context.Context) error {
	if err := k.hooks.Fire("kernel.before_start", ctx); err != nil {
		return fmt.Errorf("kernel.before_start: %w", err)
	}

	for _, m := range k.modules {
		if !k.config.ModuleEnabled(m.Name()) {
			continue
		}

		k.logger.Info("starting module", "module", m.Name())
		if err := m.Start(ctx); err != nil {
			return fmt.Errorf("start %s: %w", m.Name(), err)
		}
	}

	if err := k.hooks.Fire("kernel.after_start", ctx); err != nil {
		return fmt.Errorf("kernel.after_start: %w", err)
	}

	return nil
}

// Stop gracefully shuts down all enabled modules in reverse registration order.
//
// Lifecycle:
//  1. Fire kernel.before_stop hooks
//  2. Stop each enabled module (in reverse registration order)
//  3. Fire kernel.after_stop hooks
func (k *Kernel) Stop(ctx context.Context) error {
	if err := k.hooks.Fire("kernel.before_stop", ctx); err != nil {
		return fmt.Errorf("kernel.before_stop: %w", err)
	}

	// Stop in reverse order.
	for i := len(k.modules) - 1; i >= 0; i-- {
		m := k.modules[i]
		if !k.config.ModuleEnabled(m.Name()) {
			continue
		}

		k.logger.Info("stopping module", "module", m.Name())
		if err := m.Stop(ctx); err != nil {
			// Log but continue stopping remaining modules.
			k.logger.Error("failed to stop module", "module", m.Name(), "error", err)
		}
	}

	if err := k.hooks.Fire("kernel.after_stop", ctx); err != nil {
		return fmt.Errorf("kernel.after_stop: %w", err)
	}

	return nil
}

// Hook registers a hook handler at the named hook point.
// If called during module Init, the hook is automatically associated
// with the initializing module. Otherwise, it is associated with "kernel".
func (k *Kernel) Hook(name string, priority int, handler HookFunc) {
	module := k.initModule
	if module == "" {
		module = "kernel"
	}
	k.hooks.On(name, module, priority, handler)
	k.logger.Debug("hook registered", "point", name, "module", module, "priority", priority)
}

// HookResolve registers a resolution hook handler at the named hook point.
// Resolution hooks can short-circuit by returning resolved=true.
func (k *Kernel) HookResolve(name string, priority int, handler ResolveFunc) {
	module := k.initModule
	if module == "" {
		module = "kernel"
	}
	k.hooks.OnResolve(name, module, priority, handler)
	k.logger.Debug("resolve hook registered", "point", name, "module", module, "priority", priority)
}

// Fire executes all hooks at the named point. Convenience method that
// delegates to the hook bus.
func (k *Kernel) Fire(name string, ctx context.Context) error {
	return k.hooks.Fire(name, ctx)
}

// Resolve executes resolution hooks at the named point, returning the first
// resolved value. Convenience method that delegates to the hook bus.
func (k *Kernel) Resolve(name string, ctx context.Context) (any, error) {
	return k.hooks.Resolve(name, ctx)
}

// Provide registers a named resource in the service locator.
// Resources are available to all modules after registration.
func (k *Kernel) Provide(key string, value any) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.resources[key] = value
	k.logger.Debug("resource provided", "key", key)
}

// Resource retrieves a named resource from the service locator.
// Returns the resource and true if found, or nil and false if not.
func (k *Kernel) Resource(key string) (any, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.resources[key]
	return v, ok
}

// Logger returns the kernel's logger.
func (k *Kernel) Logger() Logger {
	return k.logger
}

// SetLogger replaces the kernel's logger. This is typically called by the
// logging module during Init to reconfigure the logger based on loaded config.
func (k *Kernel) SetLogger(l Logger) {
	k.logger = l
}

// Config returns the live configuration. Use Snapshot() on the returned
// Config to create an immutable copy for request-scoped use.
func (k *Kernel) Config() *Config {
	return k.config
}

// HookBus returns the kernel's hook bus for direct access.
func (k *Kernel) HookBus() *HookBus {
	return k.hooks
}

// GetResource is a generic helper for type-safe resource retrieval from the
// service locator. Returns an error if the resource is not found or if the
// type assertion fails.
func GetResource[T any](k *Kernel, key string) (T, error) {
	v, ok := k.Resource(key)
	if !ok {
		var zero T
		return zero, fmt.Errorf("resource not found: %s", key)
	}
	t, ok := v.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("resource %q: expected %T, got %T", key, zero, v)
	}
	return t, nil
}
