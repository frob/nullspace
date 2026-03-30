package kernel

import (
	"context"
	"fmt"
	"sync"
)

type contextKey struct{ name string }

var snapshotKey = contextKey{"config.snapshot"}

// Config holds the live, mutable configuration for the kernel.
// It is thread-safe for concurrent reads and writes. Changes to live
// config take effect at the next request boundary via Snapshot().
type Config struct {
	mu      sync.RWMutex
	modules map[string]bool // module name -> enabled
	values  map[string]any  // section key -> config value
}

// NewConfig creates an empty live configuration.
func NewConfig() *Config {
	return &Config{
		modules: make(map[string]bool),
		values:  make(map[string]any),
	}
}

// SetModuleEnabled sets the enabled state for a module by name.
func (c *Config) SetModuleEnabled(name string, enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modules[name] = enabled
}

// ModuleEnabled returns whether a module is enabled. Returns true
// if the module has no explicit entry (unknown modules default to enabled
// to support hooks registered outside of modules, e.g., by the kernel itself).
func (c *Config) ModuleEnabled(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	enabled, ok := c.modules[name]
	if !ok {
		return true
	}
	return enabled
}

// Set stores a config value at the given section key.
func (c *Config) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

// Get retrieves a config value by section key.
func (c *Config) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	return v, ok
}

// Snapshot creates an immutable copy of the current configuration.
// The snapshot is safe to use concurrently without locking.
func (c *Config) Snapshot() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	modules := make(map[string]bool, len(c.modules))
	for k, v := range c.modules {
		modules[k] = v
	}

	values := make(map[string]any, len(c.values))
	for k, v := range c.values {
		values[k] = v
	}

	return &Snapshot{
		modules: modules,
		values:  values,
	}
}

// Snapshot is an immutable configuration snapshot attached to a request context.
// Once created, it cannot be modified. Two concurrent requests may hold
// different snapshots if the live config changed between their starts.
type Snapshot struct {
	modules map[string]bool
	values  map[string]any
}

// ModuleEnabled returns whether a module is enabled in this snapshot.
// Unknown modules default to enabled.
func (s *Snapshot) ModuleEnabled(name string) bool {
	if s == nil {
		return true
	}
	enabled, ok := s.modules[name]
	if !ok {
		return true
	}
	return enabled
}

// Get retrieves a config value by section key from the snapshot.
func (s *Snapshot) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	v, ok := s.values[key]
	return v, ok
}

// Decode unmarshals the config value at the given section key into the target
// struct. The target must be a pointer. Uses a JSON round-trip internally,
// so struct fields should have `json` tags matching the TOML key names.
func (c *Config) Decode(key string, target any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	if !ok {
		return fmt.Errorf("config section not found: %s", key)
	}
	return decode(v, target)
}

// Decode unmarshals the config value at the given section key into the target
// struct. The target must be a pointer.
func (s *Snapshot) Decode(key string, target any) error {
	if s == nil {
		return fmt.Errorf("nil snapshot")
	}
	v, ok := s.values[key]
	if !ok {
		return fmt.Errorf("config section not found: %s", key)
	}
	return decode(v, target)
}

// SnapshotFromContext extracts the config snapshot from a context.
// Returns nil if no snapshot is attached.
func SnapshotFromContext(ctx context.Context) *Snapshot {
	snap, _ := ctx.Value(snapshotKey).(*Snapshot)
	return snap
}

// ContextWithSnapshot returns a new context carrying the given config snapshot.
func ContextWithSnapshot(ctx context.Context, snap *Snapshot) context.Context {
	return context.WithValue(ctx, snapshotKey, snap)
}
