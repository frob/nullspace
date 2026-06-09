package jobs

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"
)

// TestMemoryStore_EnqueueGeneratesIDIfMissing verifies that when the caller
// enqueues a Job without an ID, the memory store assigns a non-empty ID and
// the assigned ID is what Get returns.
func TestMemoryStore_EnqueueGeneratesIDIfMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := NewMemoryStore()

	j := &Job{
		Type:  "t",
		Queue: "default",
	}
	if err := s.Enqueue(ctx, j); err != nil {
		t.Fatalf("Enqueue: unexpected error: %v", err)
	}
	if j.ID == "" {
		t.Fatal("Enqueue: expected ID to be assigned, got empty")
	}

	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got.ID != j.ID {
		t.Errorf("Get: got ID %q, want %q", got.ID, j.ID)
	}
}

// TestMemoryStore_EnqueueRespectsExistingID verifies that when the caller
// pre-sets the ID, the memory store preserves it.
func TestMemoryStore_EnqueueRespectsExistingID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := NewMemoryStore()

	const fixedID = "fixed-id-12345"
	j := &Job{
		ID:    fixedID,
		Type:  "t",
		Queue: "default",
	}
	if err := s.Enqueue(ctx, j); err != nil {
		t.Fatalf("Enqueue: unexpected error: %v", err)
	}
	if j.ID != fixedID {
		t.Fatalf("Enqueue: expected ID to be preserved, got %q", j.ID)
	}

	got, err := s.Get(ctx, fixedID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got.ID != fixedID {
		t.Errorf("Get: got ID %q, want %q", got.ID, fixedID)
	}
}

// TestMemoryStore_LeaseReturnsCopy verifies that the memory store returns
// a defensive copy from Lease: mutating it does not affect subsequent
// reads through Get.
func TestMemoryStore_LeaseReturnsCopy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := NewMemoryStore()

	j := &Job{
		Type:    "t",
		Queue:   "q",
		Payload: []byte(`{"k":"v"}`),
	}
	if err := s.Enqueue(ctx, j); err != nil {
		t.Fatalf("Enqueue: unexpected error: %v", err)
	}

	leased, err := s.Lease(ctx, "w1", []string{"q"}, 30*time.Second, 1)
	if err != nil {
		t.Fatalf("Lease: unexpected error: %v", err)
	}
	if len(leased) != 1 {
		t.Fatalf("Lease: got %d jobs, want 1", len(leased))
	}

	// Mutate the returned copy.
	leased[0].LastError = "mutated-from-caller"
	leased[0].Attempts = 99
	if len(leased[0].Payload) > 0 {
		leased[0].Payload[0] = 'X'
	}

	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got.LastError == "mutated-from-caller" {
		t.Error("Get: LastError leaked through caller mutation")
	}
	if got.Attempts == 99 {
		t.Error("Get: Attempts leaked through caller mutation")
	}
	if len(got.Payload) > 0 && got.Payload[0] == 'X' {
		t.Error("Get: Payload bytes leaked through caller mutation")
	}
}

// TestMemoryStore_HeapOrderingByRunAt verifies that Lease returns ready
// jobs in RunAt-ascending order regardless of enqueue order.
func TestMemoryStore_HeapOrderingByRunAt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := NewMemoryStore()

	base := time.Now().Add(-time.Hour) // ensure all are ready (in the past)
	// Build a shuffled set of RunAt offsets.
	offsets := []time.Duration{
		0,
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
		4 * time.Second,
		5 * time.Second,
		6 * time.Second,
		7 * time.Second,
	}
	r := rand.New(rand.NewPCG(1, 2))
	r.Shuffle(len(offsets), func(i, j int) { offsets[i], offsets[j] = offsets[j], offsets[i] })

	want := make([]time.Time, 0, len(offsets))
	for _, o := range offsets {
		runAt := base.Add(o)
		want = append(want, runAt)
		j := &Job{
			Type:  "t",
			Queue: "q",
			RunAt: runAt,
		}
		if err := s.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue: unexpected error: %v", err)
		}
	}

	// Sort the want slice ascending so we can compare.
	for i := 0; i < len(want); i++ {
		for j := i + 1; j < len(want); j++ {
			if want[j].Before(want[i]) {
				want[i], want[j] = want[j], want[i]
			}
		}
	}

	got, err := s.Lease(ctx, "w1", []string{"q"}, 30*time.Second, 100)
	if err != nil {
		t.Fatalf("Lease: unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Lease: got %d jobs, want %d", len(got), len(want))
	}
	for i := range got {
		if !got[i].RunAt.Equal(want[i]) {
			t.Errorf("Lease[%d]: RunAt got %v, want %v", i, got[i].RunAt, want[i])
		}
	}
}
