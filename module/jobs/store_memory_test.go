package jobs

import "testing"

// TestMemoryStoreConformance runs the full Store contract suite against
// the in-process memory implementation.
func TestMemoryStoreConformance(t *testing.T) {
	t.Parallel()
	RunStoreConformance(t, func(t *testing.T) (Store, func()) {
		return NewMemoryStore(), func() {}
	})
}
