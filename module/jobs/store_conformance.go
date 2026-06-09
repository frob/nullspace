package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// RunStoreConformance runs the full Store-contract suite against a Store
// produced by factory. The factory is called once per subtest, must return
// a fresh empty Store, and its cleanup is invoked at subtest end.
//
// This helper is shared by every Store implementation (memory, SQL, etc.)
// to guarantee they observe the same external contract.
func RunStoreConformance(t *testing.T, factory func(t *testing.T) (Store, func())) {
	t.Helper()

	t.Run("EnqueueThenGet_AppliesDefaults", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		before := time.Now()
		j := &Job{
			Type:    "send_email",
			Payload: []byte(`{"to":"a@b"}`),
		}
		if err := s.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue: unexpected error: %v", err)
		}
		if j.ID == "" {
			t.Fatal("Enqueue: expected an ID to be assigned")
		}

		got, err := s.Get(ctx, j.ID)
		if err != nil {
			t.Fatalf("Get: unexpected error: %v", err)
		}
		if got.Type != "send_email" {
			t.Errorf("Type: got %q, want %q", got.Type, "send_email")
		}
		if got.Status != StatusPending {
			t.Errorf("Status: got %q, want %q (default pending)", got.Status, StatusPending)
		}
		if got.Queue != "default" {
			t.Errorf("Queue: got %q, want %q (default queue)", got.Queue, "default")
		}
		if got.RunAt.IsZero() {
			t.Error("RunAt: expected default to time.Now(), got zero")
		}
		// RunAt default should be within 5 seconds of "now".
		if delta := got.RunAt.Sub(before); delta < -5*time.Second || delta > 5*time.Second {
			t.Errorf("RunAt: default outside 5s window of now: delta=%v", delta)
		}
	})

	t.Run("GetMissing_ReturnsErrJobNotFound", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		_, err := s.Get(ctx, "does-not-exist-id")
		if err == nil {
			t.Fatal("Get(missing): expected error, got nil")
		}
		if !errors.Is(err, ErrJobNotFound) {
			t.Errorf("Get(missing): expected errors.Is(_, ErrJobNotFound), got %v", err)
		}
	})

	t.Run("Lease_BasicFlow", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j := &Job{Type: "t", Queue: "q1"}
		if err := s.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue: unexpected error: %v", err)
		}

		before := time.Now()
		const leaseFor = 30 * time.Second
		leased, err := s.Lease(ctx, "w1", []string{"q1"}, leaseFor, 1)
		if err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if len(leased) != 1 {
			t.Fatalf("Lease: got %d jobs, want 1", len(leased))
		}
		got := leased[0]
		if got.ID != j.ID {
			t.Errorf("Lease: ID got %q, want %q", got.ID, j.ID)
		}
		if got.Status != StatusLeased {
			t.Errorf("Lease: Status got %q, want %q", got.Status, StatusLeased)
		}
		if got.LockedBy != "w1" {
			t.Errorf("Lease: LockedBy got %q, want %q", got.LockedBy, "w1")
		}
		expectedUntil := before.Add(leaseFor)
		if delta := got.LockedUntil.Sub(expectedUntil); delta < -2*time.Second || delta > 2*time.Second {
			t.Errorf("Lease: LockedUntil outside 2s window of now+leaseFor: delta=%v", delta)
		}
	})

	t.Run("Lease_RespectsQueueFilter", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j1 := &Job{Type: "t", Queue: "q1"}
		j2 := &Job{Type: "t", Queue: "q2"}
		mustEnqueue(t, s, j1)
		mustEnqueue(t, s, j2)

		leased, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 10)
		if err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if len(leased) != 1 {
			t.Fatalf("Lease: got %d jobs, want 1", len(leased))
		}
		if leased[0].Queue != "q1" {
			t.Errorf("Lease: Queue got %q, want %q", leased[0].Queue, "q1")
		}
		if leased[0].ID != j1.ID {
			t.Errorf("Lease: ID got %q, want %q", leased[0].ID, j1.ID)
		}
	})

	t.Run("Lease_RespectsMax", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			mustEnqueue(t, s, &Job{Type: "t", Queue: "q1"})
		}

		leased, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 3)
		if err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if len(leased) != 3 {
			t.Fatalf("Lease: got %d jobs, want 3", len(leased))
		}
	})

	t.Run("Lease_SkipsFutureScheduled", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		mustEnqueue(t, s, &Job{Type: "t", Queue: "q1", RunAt: time.Now().Add(time.Hour)})

		leased, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 10)
		if err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if len(leased) != 0 {
			t.Fatalf("Lease: got %d jobs, want 0", len(leased))
		}
	})

	t.Run("Lease_SkipsAlreadyLeased", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		mustEnqueue(t, s, &Job{Type: "t", Queue: "q1"})

		first, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 10)
		if err != nil {
			t.Fatalf("Lease#1: unexpected error: %v", err)
		}
		if len(first) != 1 {
			t.Fatalf("Lease#1: got %d jobs, want 1", len(first))
		}

		second, err := s.Lease(ctx, "w2", []string{"q1"}, 30*time.Second, 10)
		if err != nil {
			t.Fatalf("Lease#2: unexpected error: %v", err)
		}
		if len(second) != 0 {
			t.Fatalf("Lease#2: got %d jobs, want 0", len(second))
		}
	})

	t.Run("Lease_ExpiredBecomesAvailable", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		mustEnqueue(t, s, &Job{Type: "t", Queue: "q1"})

		first, err := s.Lease(ctx, "w1", []string{"q1"}, time.Millisecond, 1)
		if err != nil {
			t.Fatalf("Lease#1: unexpected error: %v", err)
		}
		if len(first) != 1 {
			t.Fatalf("Lease#1: got %d jobs, want 1", len(first))
		}

		time.Sleep(20 * time.Millisecond)

		second, err := s.Lease(ctx, "w2", []string{"q1"}, 30*time.Second, 1)
		if err != nil {
			t.Fatalf("Lease#2: unexpected error: %v", err)
		}
		if len(second) != 1 {
			t.Fatalf("Lease#2 (after expiry): got %d jobs, want 1", len(second))
		}
		if second[0].LockedBy != "w2" {
			t.Errorf("Lease#2: LockedBy got %q, want %q", second[0].LockedBy, "w2")
		}
	})

	t.Run("Ack_MarksDoneAndKeepsRow", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, j)

		leased, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 1)
		if err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if len(leased) != 1 {
			t.Fatalf("Lease: got %d jobs, want 1", len(leased))
		}

		if err := s.Ack(ctx, j.ID); err != nil {
			t.Fatalf("Ack: unexpected error: %v", err)
		}

		got, err := s.Get(ctx, j.ID)
		if err != nil {
			t.Fatalf("Get: expected job to remain after Ack, got %v", err)
		}
		if got.Status != StatusDone {
			t.Errorf("Get: Status got %q, want %q", got.Status, StatusDone)
		}

		// List filtered by [StatusDone] should still see it.
		listed, err := s.List(ctx, ListFilter{Statuses: []JobStatus{StatusDone}})
		if err != nil {
			t.Fatalf("List: unexpected error: %v", err)
		}
		if !containsID(listed, j.ID) {
			t.Errorf("List(Done): expected to find acked job %q", j.ID)
		}
	})

	t.Run("Nack_ReschedulesWithRetryTime", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, j)

		if _, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 1); err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}

		retryAt := time.Now().Add(50 * time.Millisecond)
		if err := s.Nack(ctx, j.ID, retryAt, "boom"); err != nil {
			t.Fatalf("Nack: unexpected error: %v", err)
		}

		got, err := s.Get(ctx, j.ID)
		if err != nil {
			t.Fatalf("Get: unexpected error: %v", err)
		}
		if got.Status != StatusPending {
			t.Errorf("Get: Status got %q, want %q", got.Status, StatusPending)
		}
		if got.Attempts != 1 {
			t.Errorf("Get: Attempts got %d, want 1", got.Attempts)
		}
		if got.LastError != "boom" {
			t.Errorf("Get: LastError got %q, want %q", got.LastError, "boom")
		}
		if !got.RunAt.Equal(retryAt) {
			t.Errorf("Get: RunAt got %v, want %v", got.RunAt, retryAt)
		}

		// Before retryAt: lease returns nothing.
		early, err := s.Lease(ctx, "w2", []string{"q1"}, 30*time.Second, 1)
		if err != nil {
			t.Fatalf("Lease (early): unexpected error: %v", err)
		}
		if len(early) != 0 {
			t.Fatalf("Lease (early): got %d jobs, want 0", len(early))
		}

		// Wait past retryAt and try again.
		time.Sleep(80 * time.Millisecond)
		late, err := s.Lease(ctx, "w3", []string{"q1"}, 30*time.Second, 1)
		if err != nil {
			t.Fatalf("Lease (late): unexpected error: %v", err)
		}
		if len(late) != 1 {
			t.Fatalf("Lease (late): got %d jobs, want 1", len(late))
		}
	})

	t.Run("Fail_MarksTerminalAndSkipped", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, j)

		if _, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 1); err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if err := s.Fail(ctx, j.ID, "dead"); err != nil {
			t.Fatalf("Fail: unexpected error: %v", err)
		}

		got, err := s.Get(ctx, j.ID)
		if err != nil {
			t.Fatalf("Get: unexpected error: %v", err)
		}
		if got.Status != StatusFailed {
			t.Errorf("Get: Status got %q, want %q", got.Status, StatusFailed)
		}
		if got.LastError != "dead" {
			t.Errorf("Get: LastError got %q, want %q", got.LastError, "dead")
		}

		leased, err := s.Lease(ctx, "w2", []string{"q1"}, 30*time.Second, 10)
		if err != nil {
			t.Fatalf("Lease (after fail): unexpected error: %v", err)
		}
		if len(leased) != 0 {
			t.Fatalf("Lease (after fail): got %d jobs, want 0", len(leased))
		}
	})

	t.Run("Cancel_PendingSucceeds", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, j)

		if err := s.Cancel(ctx, j.ID); err != nil {
			t.Fatalf("Cancel: unexpected error: %v", err)
		}

		got, err := s.Get(ctx, j.ID)
		if err != nil {
			t.Fatalf("Get: unexpected error: %v", err)
		}
		if got.Status != StatusCancelled {
			t.Errorf("Get: Status got %q, want %q", got.Status, StatusCancelled)
		}

		leased, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 10)
		if err != nil {
			t.Fatalf("Lease (after cancel): unexpected error: %v", err)
		}
		if len(leased) != 0 {
			t.Fatalf("Lease (after cancel): got %d jobs, want 0", len(leased))
		}
	})

	t.Run("Cancel_NonPendingReturnsErrJobNotPending", func(t *testing.T) {
		ctx := context.Background()

		newLeased := func(t *testing.T) (Store, *Job, func()) {
			t.Helper()
			s, cleanup := factory(t)
			j := &Job{Type: "t", Queue: "q1"}
			mustEnqueue(t, s, j)
			if _, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 1); err != nil {
				t.Fatalf("Lease: unexpected error: %v", err)
			}
			return s, j, cleanup
		}

		t.Run("Leased", func(t *testing.T) {
			s, j, cleanup := newLeased(t)
			t.Cleanup(cleanup)

			err := s.Cancel(ctx, j.ID)
			if !errors.Is(err, ErrJobNotPending) {
				t.Fatalf("Cancel(leased): expected errors.Is(_, ErrJobNotPending), got %v", err)
			}
		})

		t.Run("Done", func(t *testing.T) {
			s, j, cleanup := newLeased(t)
			t.Cleanup(cleanup)
			if err := s.Ack(ctx, j.ID); err != nil {
				t.Fatalf("Ack: unexpected error: %v", err)
			}
			err := s.Cancel(ctx, j.ID)
			if !errors.Is(err, ErrJobNotPending) {
				t.Fatalf("Cancel(done): expected errors.Is(_, ErrJobNotPending), got %v", err)
			}
		})

		t.Run("Failed", func(t *testing.T) {
			s, j, cleanup := newLeased(t)
			t.Cleanup(cleanup)
			if err := s.Fail(ctx, j.ID, "dead"); err != nil {
				t.Fatalf("Fail: unexpected error: %v", err)
			}
			err := s.Cancel(ctx, j.ID)
			if !errors.Is(err, ErrJobNotPending) {
				t.Fatalf("Cancel(failed): expected errors.Is(_, ErrJobNotPending), got %v", err)
			}
		})

		t.Run("Cancelled", func(t *testing.T) {
			s, cleanup := factory(t)
			t.Cleanup(cleanup)
			j := &Job{Type: "t", Queue: "q1"}
			mustEnqueue(t, s, j)
			if err := s.Cancel(ctx, j.ID); err != nil {
				t.Fatalf("Cancel#1: unexpected error: %v", err)
			}
			err := s.Cancel(ctx, j.ID)
			if !errors.Is(err, ErrJobNotPending) {
				t.Fatalf("Cancel(cancelled): expected errors.Is(_, ErrJobNotPending), got %v", err)
			}
		})
	})

	t.Run("List_FiltersByStatus", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		// 3 pending.
		for i := 0; i < 3; i++ {
			mustEnqueue(t, s, &Job{Type: "t", Queue: "q1"})
		}
		// 1 done = enqueue + lease + ack.
		jDone := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, jDone)
		// 1 failed = enqueue + lease + fail.
		jFail := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, jFail)

		// Lease 2 jobs (pick the two we care about: done & fail). To keep this
		// deterministic for both heap and SQL implementations, we lease all
		// available, then ack/fail by ID.
		_, err := s.Lease(ctx, "w1", []string{"q1"}, 30*time.Second, 100)
		if err != nil {
			t.Fatalf("Lease: unexpected error: %v", err)
		}
		if err := s.Ack(ctx, jDone.ID); err != nil {
			t.Fatalf("Ack: unexpected error: %v", err)
		}
		if err := s.Fail(ctx, jFail.ID, "dead"); err != nil {
			t.Fatalf("Fail: unexpected error: %v", err)
		}
		// Nack the rest back to pending so we have 3 pending again.
		retry := time.Now().Add(-time.Hour)
		all, err := s.List(ctx, ListFilter{Statuses: []JobStatus{StatusLeased}})
		if err != nil {
			t.Fatalf("List(Leased): unexpected error: %v", err)
		}
		for _, j := range all {
			if err := s.Nack(ctx, j.ID, retry, ""); err != nil {
				t.Fatalf("Nack: unexpected error: %v", err)
			}
		}

		pending, err := s.List(ctx, ListFilter{Statuses: []JobStatus{StatusPending}})
		if err != nil {
			t.Fatalf("List(Pending): unexpected error: %v", err)
		}
		if len(pending) != 3 {
			t.Errorf("List(Pending): got %d, want 3", len(pending))
		}

		doneFailed, err := s.List(ctx, ListFilter{Statuses: []JobStatus{StatusDone, StatusFailed}})
		if err != nil {
			t.Fatalf("List(Done,Failed): unexpected error: %v", err)
		}
		if len(doneFailed) != 2 {
			t.Errorf("List(Done,Failed): got %d, want 2", len(doneFailed))
		}

		all2, err := s.List(ctx, ListFilter{})
		if err != nil {
			t.Fatalf("List({}): unexpected error: %v", err)
		}
		if len(all2) != 5 {
			t.Errorf("List({}): got %d, want 5", len(all2))
		}
	})

	t.Run("List_FiltersByQueue", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		for i := 0; i < 2; i++ {
			mustEnqueue(t, s, &Job{Type: "t", Queue: "q1"})
		}
		for i := 0; i < 3; i++ {
			mustEnqueue(t, s, &Job{Type: "t", Queue: "q2"})
		}

		got, err := s.List(ctx, ListFilter{Queue: "q1"})
		if err != nil {
			t.Fatalf("List(q1): unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("List(q1): got %d, want 2", len(got))
		}
		for _, j := range got {
			if j.Queue != "q1" {
				t.Errorf("List(q1): unexpected Queue %q", j.Queue)
			}
		}

		got2, err := s.List(ctx, ListFilter{Queue: "q2"})
		if err != nil {
			t.Fatalf("List(q2): unexpected error: %v", err)
		}
		if len(got2) != 3 {
			t.Errorf("List(q2): got %d, want 3", len(got2))
		}
	})

	t.Run("List_FiltersByType", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		for i := 0; i < 2; i++ {
			mustEnqueue(t, s, &Job{Type: "alpha", Queue: "q"})
		}
		for i := 0; i < 4; i++ {
			mustEnqueue(t, s, &Job{Type: "beta", Queue: "q"})
		}

		got, err := s.List(ctx, ListFilter{Type: "alpha"})
		if err != nil {
			t.Fatalf("List(alpha): unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("List(alpha): got %d, want 2", len(got))
		}
		for _, j := range got {
			if j.Type != "alpha" {
				t.Errorf("List(alpha): unexpected Type %q", j.Type)
			}
		}

		got2, err := s.List(ctx, ListFilter{Type: "beta"})
		if err != nil {
			t.Fatalf("List(beta): unexpected error: %v", err)
		}
		if len(got2) != 4 {
			t.Errorf("List(beta): got %d, want 4", len(got2))
		}
	})

	t.Run("List_RespectsLimit", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			mustEnqueue(t, s, &Job{Type: "t", Queue: "q"})
		}

		got, err := s.List(ctx, ListFilter{Limit: 4})
		if err != nil {
			t.Fatalf("List(Limit=4): unexpected error: %v", err)
		}
		if len(got) != 4 {
			t.Errorf("List(Limit=4): got %d, want 4", len(got))
		}
	})

	t.Run("Operations_OnMissingIDsReturnErrJobNotFound", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		const missing = "missing-random-id-9999"

		if err := s.Ack(ctx, missing); !errors.Is(err, ErrJobNotFound) {
			t.Errorf("Ack(missing): expected errors.Is(_, ErrJobNotFound), got %v", err)
		}
		if err := s.Nack(ctx, missing, time.Now(), "x"); !errors.Is(err, ErrJobNotFound) {
			t.Errorf("Nack(missing): expected errors.Is(_, ErrJobNotFound), got %v", err)
		}
		if err := s.Fail(ctx, missing, "x"); !errors.Is(err, ErrJobNotFound) {
			t.Errorf("Fail(missing): expected errors.Is(_, ErrJobNotFound), got %v", err)
		}
		if err := s.Cancel(ctx, missing); !errors.Is(err, ErrJobNotFound) {
			t.Errorf("Cancel(missing): expected errors.Is(_, ErrJobNotFound), got %v", err)
		}
	})

	t.Run("ConcurrentLease_AtMostOnceDelivery", func(t *testing.T) {
		s, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := context.Background()

		j := &Job{Type: "t", Queue: "q1"}
		mustEnqueue(t, s, j)

		const N = 10
		var wg sync.WaitGroup
		wg.Add(N)
		start := make(chan struct{})
		var winners atomic.Int32
		var empties atomic.Int32
		errs := make(chan error, N)

		for i := 0; i < N; i++ {
			workerID := i
			go func() {
				defer wg.Done()
				<-start
				leased, err := s.Lease(ctx, workerName(workerID), []string{"q1"}, 30*time.Second, 1)
				if err != nil {
					errs <- err
					return
				}
				if len(leased) == 1 {
					winners.Add(1)
				} else if len(leased) == 0 {
					empties.Add(1)
				} else {
					errs <- errTooMany(len(leased))
				}
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("Lease (concurrent): %v", err)
		}
		if w := winners.Load(); w != 1 {
			t.Errorf("winners: got %d, want 1", w)
		}
		if e := empties.Load(); e != N-1 {
			t.Errorf("empties: got %d, want %d", e, N-1)
		}
	})
}

// mustEnqueue is a small test helper used by the conformance suite.
func mustEnqueue(t *testing.T, s Store, j *Job) {
	t.Helper()
	if err := s.Enqueue(context.Background(), j); err != nil {
		t.Fatalf("Enqueue: unexpected error: %v", err)
	}
}

func containsID(jobs []*Job, id string) bool {
	for _, j := range jobs {
		if j.ID == id {
			return true
		}
	}
	return false
}

func workerName(i int) string {
	// Avoid pulling in strconv/fmt for a trivial helper.
	const digits = "0123456789"
	if i == 0 {
		return "w0"
	}
	out := []byte{'w'}
	var buf [10]byte
	n := 0
	for i > 0 {
		buf[n] = digits[i%10]
		i /= 10
		n++
	}
	for j := n - 1; j >= 0; j-- {
		out = append(out, buf[j])
	}
	return string(out)
}

// errTooMany is the error returned when concurrent Lease somehow returns
// more than one job to a single worker.
type errTooMany int

func (e errTooMany) Error() string {
	return "lease returned more than one job in single-job scenario"
}
