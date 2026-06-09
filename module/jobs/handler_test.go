package jobs

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestHandlerRegistry_HandleAndLookup(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	var called int32

	reg.Handle("x", func(ctx context.Context, job *Job) error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	h, err := reg.Lookup("x")
	if err != nil {
		t.Fatalf("Lookup(\"x\"): unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("Lookup(\"x\"): expected non-nil handler")
	}

	if err := h(context.Background(), &Job{ID: "1", Type: "x"}); err != nil {
		t.Fatalf("handler invocation: unexpected error: %v", err)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected handler to be called once, got %d", atomic.LoadInt32(&called))
	}

	if !reg.Has("x") {
		t.Fatal("Has(\"x\"): expected true after Handle")
	}
}

func TestHandlerRegistry_LookupMissing(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	h, err := reg.Lookup("missing")
	if err == nil {
		t.Fatal("Lookup(missing): expected non-nil error, got nil")
	}
	if h != nil {
		t.Fatalf("Lookup(missing): expected nil handler, got %v", h)
	}
	if reg.Has("missing") {
		t.Fatal("Has(missing): expected false")
	}
}

func TestHandlerRegistry_NamesSorted(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	noop := func(ctx context.Context, job *Job) error { return nil }

	reg.Handle("c", noop)
	reg.Handle("a", noop)
	reg.Handle("b", noop)

	got := reg.Names()
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names(): got %v, want %v", got, want)
	}
}

func TestHandlerRegistry_OverwriteReplacesHandler(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	var which int32

	reg.Handle("dup", func(ctx context.Context, job *Job) error {
		atomic.StoreInt32(&which, 1)
		return nil
	})

	// Re-register; should not panic and last-wins.
	reg.Handle("dup", func(ctx context.Context, job *Job) error {
		atomic.StoreInt32(&which, 2)
		return nil
	})

	h, err := reg.Lookup("dup")
	if err != nil {
		t.Fatalf("Lookup(dup): unexpected error: %v", err)
	}
	if err := h(context.Background(), &Job{ID: "1", Type: "dup"}); err != nil {
		t.Fatalf("handler invocation: unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&which); got != 2 {
		t.Fatalf("expected the second handler to be active (which=2), got which=%d", got)
	}

	// Only one entry should be visible from Names().
	names := reg.Names()
	count := 0
	for _, n := range names {
		if n == "dup" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected \"dup\" to appear once in Names(), got %d (names=%v)", count, names)
	}
}

func TestHandlerRegistry_HandleEmptyNamePanics(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Handle(\"\", fn): expected panic, got none")
		}
	}()

	reg.Handle("", func(ctx context.Context, job *Job) error { return nil })
}

func TestHandlerRegistry_HandleNilHandlerPanics(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Handle(\"x\", nil): expected panic, got none")
		}
	}()

	reg.Handle("x", nil)
}

func TestHandlerRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	reg := NewHandlerRegistry()
	const goroutines = 32
	const perGoroutine = 50

	noop := func(ctx context.Context, job *Job) error { return nil }

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				name := nameFor(g, i)
				reg.Handle(name, noop)
			}
		}()
	}

	// Readers
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				name := nameFor(g, i)
				// We don't care if it's there yet — we just want the read path
				// to be safe under concurrent writes (race detector will yell
				// if there's no lock).
				_, _ = reg.Lookup(name)
				_ = reg.Has(name)
				_ = reg.Names()
			}
		}()
	}

	wg.Wait()
}

// nameFor builds a deterministic handler name for the concurrent test.
func nameFor(g, i int) string {
	t := []byte{'h', byte('a' + (g % 26)), byte('0' + (i % 10))}
	return string(t)
}
