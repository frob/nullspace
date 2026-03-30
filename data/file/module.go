package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/frob/nullspace/kernel"
)

// Config holds the file module's configuration.
type Config struct {
	Dir    string `json:"dir" toml:"dir"`
	Format string `json:"format" toml:"format"` // default format for new files
}

// Module provides file-based entity storage.
type Module struct {
	dir    string
	format string
	kernel *kernel.Kernel
}

// New creates a new file data module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "data.file" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "data.file",
		Default: Config{
			Dir:    "./content",
			Format: "markdown",
		},
		DefaultEnabled: true,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	var cfg Config
	if err := k.Config().Decode("data.file", &cfg); err != nil {
		cfg = Config{Dir: "./content", Format: "markdown"}
	}
	m.dir = cfg.Dir
	m.format = cfg.Format

	k.Provide("data.file", m)
	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

func (m *Module) Healthy(ctx context.Context) error {
	_, err := os.Stat(m.dir)
	return err
}

// Read loads a single entity from a collection by ID.
// The collection maps to a subdirectory and the ID to a filename.
// It tries known extensions (.md, .json, .toml) if no exact match.
func (m *Module) Read(ctx context.Context, collection, id string) (*Entity, error) {
	if err := m.kernel.Fire("data.before_read", ctx); err != nil {
		return nil, err
	}

	entity, err := m.readFile(collection, id)
	if err != nil {
		return nil, err
	}

	if err := m.kernel.Fire("data.after_read", ctx); err != nil {
		return nil, err
	}

	return entity, nil
}

// List returns all entities in a collection.
func (m *Module) List(ctx context.Context, collection string) ([]*Entity, error) {
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

	var entities []*Entity
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		id := strings.TrimSuffix(entry.Name(), ext)

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		entity, err := parseFile(id, ext, data)
		if err != nil {
			continue
		}
		entities = append(entities, entity)
	}

	if err := m.kernel.Fire("data.after_read", ctx); err != nil {
		return nil, err
	}

	return entities, nil
}

// Write stores an entity in a collection. The file format is determined by
// the entity's Format field, falling back to the module's default format.
func (m *Module) Write(ctx context.Context, collection, id string, entity *Entity) error {
	if err := m.kernel.Fire("data.before_write", ctx); err != nil {
		return err
	}

	format := entity.Format
	if format == "" {
		format = m.format
	}

	data, err := serializeEntity(entity, format)
	if err != nil {
		return err
	}

	dir := filepath.Join(m.dir, collection)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	ext := formatToExt(format)
	path := filepath.Join(dir, id+ext)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := m.kernel.Fire("data.after_write", ctx); err != nil {
		return err
	}

	return nil
}

// Delete removes an entity from a collection.
func (m *Module) Delete(ctx context.Context, collection, id string) error {
	if err := m.kernel.Fire("data.before_write", ctx); err != nil {
		return err
	}

	path, err := m.findFile(collection, id)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}

	if err := m.kernel.Fire("data.after_write", ctx); err != nil {
		return err
	}

	return nil
}

// readFile finds and parses a file by collection and ID.
func (m *Module) readFile(collection, id string) (*Entity, error) {
	path, err := m.findFile(collection, id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	ext := filepath.Ext(path)
	return parseFile(id, ext, data)
}

// findFile locates an entity file, trying known extensions.
func (m *Module) findFile(collection, id string) (string, error) {
	dir := filepath.Join(m.dir, collection)

	for _, ext := range []string{".md", ".markdown", ".json", ".toml"} {
		path := filepath.Join(dir, id+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("entity not found: %s/%s", collection, id)
}

func formatToExt(format string) string {
	switch format {
	case "json":
		return ".json"
	case "toml":
		return ".toml"
	default:
		return ".md"
	}
}
