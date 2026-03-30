package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/frob/nullspace/kernel"
)

func setupFileModule(t *testing.T, dir string) (*Module, *kernel.Kernel) {
	t.Helper()

	tomlPath := filepath.Join(t.TempDir(), "nullspace.toml")
	os.WriteFile(tomlPath, []byte(`
[data.file]
dir = "`+dir+`"
format = "markdown"
`), 0644)

	k := kernel.New(kernel.WithConfigFile(tomlPath))
	m := New()
	k.Use(m)

	if err := k.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return m, k
}

// --- Parser Tests ---

func TestParseMarkdownYAMLFrontmatter(t *testing.T) {
	data := []byte(`---
title: My Post
tags:
  - go
  - web
---

Hello world.`)

	entity, err := parseMarkdown("test", data)
	if err != nil {
		t.Fatalf("parseMarkdown: %v", err)
	}

	if entity.ID != "test" {
		t.Fatalf("expected id=test, got %s", entity.ID)
	}
	if entity.Meta["title"] != "My Post" {
		t.Fatalf("expected title, got %v", entity.Meta["title"])
	}
	tags, ok := entity.Meta["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", entity.Meta["tags"])
	}
	if entity.Body != "Hello world." {
		t.Fatalf("expected body, got %q", entity.Body)
	}
	if entity.Format != "markdown" {
		t.Fatalf("expected format=markdown, got %s", entity.Format)
	}
}

func TestParseMarkdownTOMLFrontmatter(t *testing.T) {
	data := []byte(`+++
title = "TOML Post"
draft = true
+++

Content here.`)

	entity, err := parseMarkdown("test", data)
	if err != nil {
		t.Fatalf("parseMarkdown: %v", err)
	}

	if entity.Meta["title"] != "TOML Post" {
		t.Fatalf("expected title, got %v", entity.Meta["title"])
	}
	if entity.Meta["draft"] != true {
		t.Fatalf("expected draft=true, got %v", entity.Meta["draft"])
	}
	if entity.Body != "Content here." {
		t.Fatalf("expected body, got %q", entity.Body)
	}
}

func TestParseMarkdownNoFrontmatter(t *testing.T) {
	data := []byte("Just some text.")

	entity, err := parseMarkdown("test", data)
	if err != nil {
		t.Fatalf("parseMarkdown: %v", err)
	}

	if len(entity.Meta) != 0 {
		t.Fatalf("expected empty meta, got %v", entity.Meta)
	}
	if entity.Body != "Just some text." {
		t.Fatalf("expected body, got %q", entity.Body)
	}
}

func TestParseJSON(t *testing.T) {
	data := []byte(`{
  "title": "JSON Post",
  "count": 42,
  "body": "The content."
}`)

	entity, err := parseJSON("test", data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}

	if entity.Meta["title"] != "JSON Post" {
		t.Fatalf("expected title, got %v", entity.Meta["title"])
	}
	if entity.Body != "The content." {
		t.Fatalf("expected body, got %q", entity.Body)
	}
	// "body" should be removed from Meta.
	if _, ok := entity.Meta["body"]; ok {
		t.Fatal("body should not be in meta")
	}
	if entity.Format != "json" {
		t.Fatalf("expected format=json, got %s", entity.Format)
	}
}

func TestParseTOML(t *testing.T) {
	data := []byte(`title = "TOML Post"
body = "Content here."
`)

	entity, err := parseTOML("test", data)
	if err != nil {
		t.Fatalf("parseTOML: %v", err)
	}

	if entity.Meta["title"] != "TOML Post" {
		t.Fatalf("expected title, got %v", entity.Meta["title"])
	}
	if entity.Body != "Content here." {
		t.Fatalf("expected body, got %q", entity.Body)
	}
	if entity.Format != "toml" {
		t.Fatalf("expected format=toml, got %s", entity.Format)
	}
}

func TestParseFileByExtension(t *testing.T) {
	mdData := []byte("---\ntitle: Test\n---\nBody")
	jsonData := []byte(`{"title":"Test"}`)
	tomlData := []byte(`title = "Test"`)

	tests := []struct {
		ext    string
		data   []byte
		format string
	}{
		{".md", mdData, "markdown"},
		{".markdown", mdData, "markdown"},
		{".json", jsonData, "json"},
		{".toml", tomlData, "toml"},
	}

	for _, tt := range tests {
		entity, err := parseFile("test", tt.ext, tt.data)
		if err != nil {
			t.Fatalf("parseFile(%s): %v", tt.ext, err)
		}
		if entity.Format != tt.format {
			t.Fatalf("parseFile(%s): expected format=%s, got %s", tt.ext, tt.format, entity.Format)
		}
	}
}

func TestParseFileUnsupported(t *testing.T) {
	_, err := parseFile("test", ".xyz", []byte("data"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

// --- Serialization Tests ---

func TestSerializeMarkdown(t *testing.T) {
	entity := &Entity{
		ID:     "test",
		Meta:   map[string]any{"title": "Hello"},
		Body:   "World",
		Format: "markdown",
	}

	data, err := serializeEntity(entity, "markdown")
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	// Round-trip: parse the serialized data back.
	parsed, err := parseMarkdown("test", data)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if parsed.Meta["title"] != "Hello" {
		t.Fatalf("round-trip title: %v", parsed.Meta["title"])
	}
	if parsed.Body != "World" {
		t.Fatalf("round-trip body: %q", parsed.Body)
	}
}

func TestSerializeJSON(t *testing.T) {
	entity := &Entity{
		ID:     "test",
		Meta:   map[string]any{"title": "Hello"},
		Body:   "World",
		Format: "json",
	}

	data, err := serializeEntity(entity, "json")
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	parsed, err := parseJSON("test", data)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if parsed.Meta["title"] != "Hello" {
		t.Fatalf("round-trip title: %v", parsed.Meta["title"])
	}
	if parsed.Body != "World" {
		t.Fatalf("round-trip body: %q", parsed.Body)
	}
}

func TestSerializeTOML(t *testing.T) {
	entity := &Entity{
		ID:     "test",
		Meta:   map[string]any{"title": "Hello"},
		Body:   "World",
		Format: "toml",
	}

	data, err := serializeEntity(entity, "toml")
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	parsed, err := parseTOML("test", data)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if parsed.Meta["title"] != "Hello" {
		t.Fatalf("round-trip title: %v", parsed.Meta["title"])
	}
	if parsed.Body != "World" {
		t.Fatalf("round-trip body: %q", parsed.Body)
	}
}

// --- CRUD Tests ---

func TestReadWriteDelete(t *testing.T) {
	dir := t.TempDir()
	m, _ := setupFileModule(t, dir)
	ctx := context.Background()

	// Write an entity.
	entity := &Entity{
		ID:     "hello",
		Meta:   map[string]any{"title": "Hello Post"},
		Body:   "Content here.",
		Format: "markdown",
	}
	if err := m.Write(ctx, "posts", "hello", entity); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, "posts", "hello.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s", path)
	}

	// Read it back.
	read, err := m.Read(ctx, "posts", "hello")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Meta["title"] != "Hello Post" {
		t.Fatalf("expected title, got %v", read.Meta["title"])
	}
	if read.Body != "Content here." {
		t.Fatalf("expected body, got %q", read.Body)
	}

	// Delete it.
	if err := m.Delete(ctx, "posts", "hello"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected file to be deleted")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m, _ := setupFileModule(t, dir)
	ctx := context.Background()

	// Write two entities.
	for _, id := range []string{"first", "second"} {
		entity := &Entity{
			ID:     id,
			Meta:   map[string]any{"title": id},
			Body:   "Body of " + id,
			Format: "markdown",
		}
		if err := m.Write(ctx, "posts", id, entity); err != nil {
			t.Fatalf("Write %s: %v", id, err)
		}
	}

	entities, err := m.List(ctx, "posts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
}

func TestListEmptyCollection(t *testing.T) {
	dir := t.TempDir()
	m, _ := setupFileModule(t, dir)
	ctx := context.Background()

	entities, err := m.List(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(entities))
	}
}

func TestReadNotFound(t *testing.T) {
	dir := t.TempDir()
	m, _ := setupFileModule(t, dir)
	ctx := context.Background()

	_, err := m.Read(ctx, "posts", "missing")
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	m, _ := setupFileModule(t, dir)
	ctx := context.Background()

	entity := &Entity{
		ID:     "data",
		Meta:   map[string]any{"key": "value"},
		Format: "json",
	}
	if err := m.Write(ctx, "config", "data", entity); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(dir, "config", "data.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected JSON file at %s", path)
	}

	read, err := m.Read(ctx, "config", "data")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Meta["key"] != "value" {
		t.Fatalf("expected key=value, got %v", read.Meta["key"])
	}
}
