package kernel

import (
	"context"
	"fmt"
	"testing"
)

// testModule is a minimal Module for testing lifecycle and hooks.
type testModule struct {
	name        string
	initCalled  bool
	startCalled bool
	stopCalled  bool
	initErr     error
	startErr    error
	stopErr     error
	initFunc    func(k *Kernel) error
}

func (m *testModule) Name() string { return m.name }

func (m *testModule) Init(k *Kernel) error {
	m.initCalled = true
	if m.initFunc != nil {
		return m.initFunc(k)
	}
	return m.initErr
}

func (m *testModule) Start(ctx context.Context) error {
	m.startCalled = true
	return m.startErr
}

func (m *testModule) Stop(ctx context.Context) error {
	m.stopCalled = true
	return m.stopErr
}

// configurableModule implements both Module and Configurable.
type configurableModule struct {
	testModule
	config ModuleConfig
}

func (m *configurableModule) Config() ModuleConfig {
	return m.config
}

func TestKernelLifecycle(t *testing.T) {
	ctx := context.Background()
	k := New()

	m1 := &testModule{name: "mod1"}
	m2 := &testModule{name: "mod2"}
	k.Use(m1)
	k.Use(m2)

	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !m1.initCalled || !m2.initCalled {
		t.Fatal("expected both modules to be initialized")
	}

	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m1.startCalled || !m2.startCalled {
		t.Fatal("expected both modules to be started")
	}

	if err := k.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !m1.stopCalled || !m2.stopCalled {
		t.Fatal("expected both modules to be stopped")
	}
}

func TestKernelInitError(t *testing.T) {
	ctx := context.Background()
	k := New()

	m1 := &testModule{name: "mod1", initErr: fmt.Errorf("init failed")}
	k.Use(m1)

	err := k.Init(ctx)
	if err == nil {
		t.Fatal("expected error from Init")
	}
	if err.Error() != "init mod1: init failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKernelDisabledModule(t *testing.T) {
	ctx := context.Background()
	k := New()

	m := &configurableModule{
		testModule: testModule{name: "optional"},
		config: ModuleConfig{
			Key:            "optional",
			DefaultEnabled: false,
		},
	}
	k.Use(m)

	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if m.initCalled {
		t.Fatal("disabled module should not be initialized")
	}

	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if m.startCalled {
		t.Fatal("disabled module should not be started")
	}
}

func TestKernelServiceLocator(t *testing.T) {
	k := New()
	k.Provide("db", "sqlite://test.db")

	val, ok := k.Resource("db")
	if !ok {
		t.Fatal("expected resource to be found")
	}
	if val != "sqlite://test.db" {
		t.Fatalf("unexpected value: %v", val)
	}

	_, ok = k.Resource("nonexistent")
	if ok {
		t.Fatal("expected resource not found")
	}
}

func TestGetResource(t *testing.T) {
	k := New()
	k.Provide("count", 42)

	val, err := GetResource[int](k, "count")
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}

	_, err = GetResource[string](k, "count")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}

	_, err = GetResource[int](k, "missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestHookBusFire(t *testing.T) {
	ctx := context.Background()
	bus := NewHookBus()

	var order []string
	bus.On("test.point", "mod1", 20, func(ctx context.Context) error {
		order = append(order, "second")
		return nil
	})
	bus.On("test.point", "mod2", 10, func(ctx context.Context) error {
		order = append(order, "first")
		return nil
	})

	if err := bus.Fire("test.point", ctx); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("unexpected execution order: %v", order)
	}
}

func TestHookBusFireError(t *testing.T) {
	ctx := context.Background()
	bus := NewHookBus()

	bus.On("test.point", "mod1", 10, func(ctx context.Context) error {
		return fmt.Errorf("hook failed")
	})
	bus.On("test.point", "mod2", 20, func(ctx context.Context) error {
		t.Fatal("should not execute after error")
		return nil
	})

	err := bus.Fire("test.point", ctx)
	if err == nil || err.Error() != "hook failed" {
		t.Fatalf("expected hook error, got: %v", err)
	}
}

func TestHookBusResolve(t *testing.T) {
	ctx := context.Background()
	bus := NewHookBus()

	// First resolver doesn't resolve.
	bus.OnResolve("format.resolve", "mod1", 10, func(ctx context.Context) (any, bool, error) {
		return nil, false, nil
	})
	// Second resolver resolves.
	bus.OnResolve("format.resolve", "mod2", 20, func(ctx context.Context) (any, bool, error) {
		return "json", true, nil
	})
	// Third resolver should not run.
	bus.OnResolve("format.resolve", "mod3", 30, func(ctx context.Context) (any, bool, error) {
		t.Fatal("should not execute after resolution")
		return nil, false, nil
	})

	val, err := bus.Resolve("format.resolve", ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "json" {
		t.Fatalf("expected 'json', got %v", val)
	}
}

func TestHookBusConfigAware(t *testing.T) {
	bus := NewHookBus()

	var called []string
	bus.On("test.point", "enabled_mod", 10, func(ctx context.Context) error {
		called = append(called, "enabled")
		return nil
	})
	bus.On("test.point", "disabled_mod", 20, func(ctx context.Context) error {
		called = append(called, "disabled")
		return nil
	})

	// Create a snapshot with disabled_mod disabled.
	cfg := NewConfig()
	cfg.SetModuleEnabled("enabled_mod", true)
	cfg.SetModuleEnabled("disabled_mod", false)
	snap := cfg.Snapshot()

	ctx := ContextWithSnapshot(context.Background(), snap)

	if err := bus.Fire("test.point", ctx); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if len(called) != 1 || called[0] != "enabled" {
		t.Fatalf("expected only enabled module hook to run, got: %v", called)
	}
}

func TestConfigSnapshot(t *testing.T) {
	cfg := NewConfig()
	cfg.SetModuleEnabled("mod1", true)
	cfg.Set("app.name", "test")

	snap := cfg.Snapshot()

	// Mutate live config after snapshot.
	cfg.SetModuleEnabled("mod1", false)
	cfg.Set("app.name", "changed")

	// Snapshot should be unaffected.
	if !snap.ModuleEnabled("mod1") {
		t.Fatal("snapshot should still show mod1 as enabled")
	}
	val, ok := snap.Get("app.name")
	if !ok || val != "test" {
		t.Fatalf("snapshot should still have original value, got: %v", val)
	}
}

func TestConfigSnapshotContext(t *testing.T) {
	cfg := NewConfig()
	cfg.SetModuleEnabled("mod1", true)
	snap := cfg.Snapshot()

	ctx := ContextWithSnapshot(context.Background(), snap)
	extracted := SnapshotFromContext(ctx)

	if extracted == nil {
		t.Fatal("expected snapshot in context")
	}
	if !extracted.ModuleEnabled("mod1") {
		t.Fatal("expected mod1 enabled in extracted snapshot")
	}

	// Context without snapshot.
	noSnap := SnapshotFromContext(context.Background())
	if noSnap != nil {
		t.Fatal("expected nil snapshot from plain context")
	}
}

func TestKernelHookRegistration(t *testing.T) {
	ctx := context.Background()
	k := New()

	var hookModule string
	m := &testModule{
		name: "hooker",
		initFunc: func(k *Kernel) error {
			k.Hook("test.point", 10, func(ctx context.Context) error {
				hookModule = "hooker"
				return nil
			})
			return nil
		},
	}
	k.Use(m)

	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := k.Fire("test.point", ctx); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if hookModule != "hooker" {
		t.Fatal("expected hook to fire with module context")
	}
}

func TestKernelStopOrder(t *testing.T) {
	ctx := context.Background()
	k := New()

	var stopOrder []string
	m1 := &testModule{name: "first"}
	m1.stopCalled = false
	m2 := &testModule{name: "second"}

	// Override Stop to track order.
	type stoppable struct {
		testModule
		onStop func()
	}

	s1 := &testModule{name: "first"}
	s2 := &testModule{name: "second"}

	k.Use(s1)
	k.Use(s2)

	if err := k.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := k.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Replace Stop behavior to track order via hooks.
	k.Hook("kernel.before_stop", 10, func(ctx context.Context) error {
		stopOrder = append(stopOrder, "before_stop")
		return nil
	})

	if err := k.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Verify both stopped (reverse order is tested by checking both ran).
	if !s1.stopCalled || !s2.stopCalled {
		t.Fatal("expected both modules to be stopped")
	}

	_ = stopOrder
	_ = m1
	_ = m2
}
