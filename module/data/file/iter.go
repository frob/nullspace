package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/frob/nullspace/kernel"
)

// ListIter lazily reads entities from a collection directory one at a time.
// Call Next() to advance and Close() when done.
type ListIter struct {
	dir        string
	collection string
	entries    []os.DirEntry
	pos        int
	total      int
	ctx        context.Context
	kernel     *kernel.Kernel
	closed     bool
}

// ListIter returns a lazy iterator over entities in a collection.
// The directory listing is read eagerly but files are parsed one at a time
// as Next() is called. Returns nil if the collection directory does not exist.
func (m *Module) ListIter(ctx context.Context, collection string) (*ListIter, error) {
	if err := m.kernel.Fire("data.before_read", ctx); err != nil {
		return nil, err
	}

	dir := filepath.Join(m.dir, collection)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list %s: %w", collection, err)
	}

	// Count files (not directories) for the Total hint.
	fileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			fileCount++
		}
	}

	return &ListIter{
		dir:        dir,
		collection: collection,
		entries:    entries,
		total:      fileCount,
		ctx:        ctx,
		kernel:     m.kernel,
	}, nil
}

// Next returns the next entity, or nil when exhausted.
// Files that fail to read or parse are skipped silently.
func (it *ListIter) Next() (*Entity, error) {
	for it.pos < len(it.entries) {
		if err := it.ctx.Err(); err != nil {
			return nil, err
		}

		entry := it.entries[it.pos]
		it.pos++

		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		id := strings.TrimSuffix(entry.Name(), ext)

		data, err := os.ReadFile(filepath.Join(it.dir, entry.Name()))
		if err != nil {
			continue
		}

		entity, err := parseFile(id, ext, data)
		if err != nil {
			continue
		}

		return entity, nil
	}

	return nil, nil
}

// Total returns the number of files in the directory listing.
// This is a hint — some files may fail to parse.
func (it *ListIter) Total() int {
	return it.total
}

// Collection returns the collection name.
func (it *ListIter) Collection() string {
	return it.collection
}

// Close fires the data.after_read hook. Must be called when done iterating.
func (it *ListIter) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	return it.kernel.Fire("data.after_read", it.ctx)
}
