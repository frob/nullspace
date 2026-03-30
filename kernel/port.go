package kernel

import "context"

// Module is the base interface for all framework components.
// Every component in the framework — request adapters, response formatters,
// data providers, middleware — implements this interface.
type Module interface {
	// Name returns a unique identifier for this module (e.g., "data.static").
	Name() string

	// Init wires the module into the kernel: registers hooks, provides resources,
	// and reads its configuration. Called only if the module is enabled.
	Init(k *Kernel) error

	// Start begins the module's runtime operation. Called after all modules
	// have been initialized.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the module. Called in reverse registration order.
	Stop(ctx context.Context) error
}

// Configurable is implemented by modules that declare a configuration section.
// The kernel collects these declarations before Init, loads config from TOML
// and environment variables, and passes the populated values back during Init.
type Configurable interface {
	// Config returns the module's config declaration: its TOML section key,
	// default values, and default enabled state.
	Config() ModuleConfig
}

// ModuleConfig declares a module's configuration section.
type ModuleConfig struct {
	// Key is the TOML section path (e.g., "data.static", "log").
	Key string

	// Default is the default config struct for this module.
	// The kernel unmarshals the TOML section into a value of this type.
	Default any

	// DefaultEnabled controls whether the module is enabled when no
	// explicit configuration is provided. Required/core modules should
	// default to true; optional modules may default to false.
	DefaultEnabled bool
}

// DataProvider extends Module with health checking for data-oriented modules.
type DataProvider interface {
	Module
	Healthy(ctx context.Context) error
}

// Logger is the framework's logging port. The interface mirrors log/slog
// semantics: structured key-value pairs with leveled output.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	// With returns a child logger with the given key-value pairs
	// permanently attached to every subsequent log entry.
	With(args ...any) Logger
}
