package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/frob/nullspace/core/nslog"
	"github.com/frob/nullspace/kernel"
)

func setupIterModule(t *testing.T) (*Module, string) {
	t.Helper()

	contentDir := t.TempDir()
	configDir := t.TempDir()
	tomlPath := filepath.Join(configDir, "nullspace.toml")
	os.WriteFile(tomlPath, []byte(`
[data.file]
dir = "`+contentDir+`"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	k.Use(nslog.New())
	m := New()
	k.Use(m)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return m, contentDir
}

func TestListIterBasic(t *testing.T) {
	m, contentDir := setupIterModule(t)

	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "a.json"), []byte(`{"title":"First"}`), 0644)
	os.WriteFile(filepath.Join(postsDir, "b.json"), []byte(`{"title":"Second"}`), 0644)

	iter, err := m.ListIter(context.Background(), "posts")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}
	if iter == nil {
		t.Fatal("expected non-nil iterator")
	}
	defer iter.Close()

	if iter.Total() != 2 {
		t.Fatalf("Total = %d, want 2", iter.Total())
	}
	if iter.Collection() != "posts" {
		t.Fatalf("Collection = %s, want posts", iter.Collection())
	}

	// Read all entities.
	var ids []string
	for {
		e, err := iter.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if e == nil {
			break
		}
		ids = append(ids, e.ID)
	}

	if len(ids) != 2 {
		t.Fatalf("got %d entities, want 2", len(ids))
	}
}

func TestListIterEmptyCollection(t *testing.T) {
	m, contentDir := setupIterModule(t)

	// Create empty collection directory.
	os.MkdirAll(filepath.Join(contentDir, "empty"), 0755)

	iter, err := m.ListIter(context.Background(), "empty")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}
	if iter == nil {
		t.Fatal("expected non-nil iterator for empty dir")
	}
	defer iter.Close()

	if iter.Total() != 0 {
		t.Fatalf("Total = %d, want 0", iter.Total())
	}

	e, err := iter.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if e != nil {
		t.Fatal("expected nil from empty iterator")
	}
}

func TestListIterMissingCollection(t *testing.T) {
	m, _ := setupIterModule(t)

	iter, err := m.ListIter(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}
	if iter != nil {
		t.Fatal("expected nil iterator for missing collection")
	}
}

func TestListIterSkipsInvalidFiles(t *testing.T) {
	m, contentDir := setupIterModule(t)

	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "good.json"), []byte(`{"title":"Good"}`), 0644)
	os.WriteFile(filepath.Join(postsDir, "bad.json"), []byte(`not valid json`), 0644)

	iter, err := m.ListIter(context.Background(), "posts")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}
	defer iter.Close()

	if iter.Total() != 2 {
		t.Fatalf("Total = %d, want 2 (includes bad file)", iter.Total())
	}

	var count int
	for {
		e, err := iter.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if e == nil {
			break
		}
		count++
	}

	if count != 1 {
		t.Fatalf("got %d valid entities, want 1", count)
	}
}

func TestListIterContextCancellation(t *testing.T) {
	m, contentDir := setupIterModule(t)

	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.WriteFile(filepath.Join(postsDir, "a.json"), []byte(`{"title":"A"}`), 0644)
	os.WriteFile(filepath.Join(postsDir, "b.json"), []byte(`{"title":"B"}`), 0644)

	ctx, cancel := context.WithCancel(context.Background())

	iter, err := m.ListIter(ctx, "posts")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}
	defer iter.Close()

	// Read one item, then cancel.
	e, err := iter.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if e == nil {
		t.Fatal("expected first entity")
	}

	cancel()

	_, err = iter.Next()
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
}

func TestListIterCloseIdempotent(t *testing.T) {
	m, contentDir := setupIterModule(t)

	os.MkdirAll(filepath.Join(contentDir, "posts"), 0755)
	os.WriteFile(filepath.Join(contentDir, "posts", "a.json"), []byte(`{}`), 0644)

	iter, err := m.ListIter(context.Background(), "posts")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}

	// Close twice — should not panic or error.
	if err := iter.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := iter.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestListIterSkipsDirectories(t *testing.T) {
	m, contentDir := setupIterModule(t)

	postsDir := filepath.Join(contentDir, "posts")
	os.MkdirAll(postsDir, 0755)
	os.MkdirAll(filepath.Join(postsDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(postsDir, "good.json"), []byte(`{"title":"Good"}`), 0644)

	iter, err := m.ListIter(context.Background(), "posts")
	if err != nil {
		t.Fatalf("ListIter: %v", err)
	}
	defer iter.Close()

	if iter.Total() != 1 {
		t.Fatalf("Total = %d, want 1 (excludes subdir)", iter.Total())
	}

	e, err := iter.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if e == nil || e.ID != "good" {
		t.Fatalf("expected entity 'good', got %v", e)
	}

	e, err = iter.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if e != nil {
		t.Fatal("expected nil after last entity")
	}
}
