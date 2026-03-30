package kernel

import (
	"context"
	"slices"
	"sync"
)

// HookFunc is a standard hook handler executed at a named hook point.
type HookFunc func(ctx context.Context) error

// ResolveFunc is a hook handler that participates in short-circuit resolution.
// It returns (value, resolved, error). If resolved is true, the chain stops
// and the value is returned to the caller.
type ResolveFunc func(ctx context.Context) (any, bool, error)

type hookEntry struct {
	module   string
	priority int
	handler  HookFunc
}

type resolveEntry struct {
	module   string
	priority int
	handler  ResolveFunc
}

// HookBus manages prioritized, config-aware hook chains.
//
// Hooks are registered at named hook points with a priority (lower = earlier).
// When fired, the bus executes handlers in priority order, skipping any whose
// owning module is disabled in the current request's config snapshot.
type HookBus struct {
	hooks     map[string][]hookEntry
	resolvers map[string][]resolveEntry
	mu        sync.RWMutex
}

// NewHookBus creates an empty hook bus.
func NewHookBus() *HookBus {
	return &HookBus{
		hooks:     make(map[string][]hookEntry),
		resolvers: make(map[string][]resolveEntry),
	}
}

// On registers a hook handler at the named hook point with the given priority.
// The module parameter identifies the owning module for config-aware filtering.
func (b *HookBus) On(name, module string, priority int, handler HookFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := hookEntry{module: module, priority: priority, handler: handler}
	b.hooks[name] = append(b.hooks[name], entry)
	slices.SortFunc(b.hooks[name], func(a, c hookEntry) int {
		return a.priority - c.priority
	})
}

// OnResolve registers a resolution handler at the named hook point.
// Resolution handlers can short-circuit the chain by returning resolved=true.
func (b *HookBus) OnResolve(name, module string, priority int, handler ResolveFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := resolveEntry{module: module, priority: priority, handler: handler}
	b.resolvers[name] = append(b.resolvers[name], entry)
	slices.SortFunc(b.resolvers[name], func(a, c resolveEntry) int {
		return a.priority - c.priority
	})
}

// Fire executes all hooks at the named point in priority order.
// If the context carries a config snapshot, hooks whose owning module
// is disabled are skipped. Execution stops at the first error.
func (b *HookBus) Fire(name string, ctx context.Context) error {
	b.mu.RLock()
	entries := make([]hookEntry, len(b.hooks[name]))
	copy(entries, b.hooks[name])
	b.mu.RUnlock()

	snap := SnapshotFromContext(ctx)

	for _, e := range entries {
		if snap != nil && !snap.ModuleEnabled(e.module) {
			continue
		}
		if err := e.handler(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Resolve executes resolution hooks in priority order, returning the first
// resolved value. If no handler resolves, returns (nil, nil).
// Disabled modules are skipped based on the context's config snapshot.
func (b *HookBus) Resolve(name string, ctx context.Context) (any, error) {
	b.mu.RLock()
	entries := make([]resolveEntry, len(b.resolvers[name]))
	copy(entries, b.resolvers[name])
	b.mu.RUnlock()

	snap := SnapshotFromContext(ctx)

	for _, e := range entries {
		if snap != nil && !snap.ModuleEnabled(e.module) {
			continue
		}
		val, resolved, err := e.handler(ctx)
		if err != nil {
			return nil, err
		}
		if resolved {
			return val, nil
		}
	}
	return nil, nil
}

// Hooks returns the names of all registered hook points. Useful for debugging.
func (b *HookBus) Hooks() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	seen := make(map[string]struct{})
	for name := range b.hooks {
		seen[name] = struct{}{}
	}
	for name := range b.resolvers {
		seen[name] = struct{}{}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
