// Package file provides file-based entity storage.
//
// Entities are stored as files on disk — one file per record. Supported
// formats are markdown (with YAML or TOML frontmatter), JSON, and TOML.
// Directory structure maps to entity type: content/posts/my-post.md
package file

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Entity represents a single stored record.
type Entity struct {
	// ID is the entity identifier, derived from the filename (without extension).
	ID string

	// Meta holds structured fields from frontmatter (markdown) or the full
	// document (JSON/TOML). Values are Go types from the parser.
	Meta map[string]any

	// Body holds the content body. For markdown, this is everything after
	// the frontmatter. For JSON/TOML, this is the value of the "body" key
	// (if present) or empty.
	Body string

	// Format is the file format: "markdown", "json", or "toml".
	Format string
}

// ValidateSegment rejects values that could cause path traversal or injection
// when used as a file path component.
func ValidateSegment(s string) error {
	if strings.ContainsAny(s, "/\\") {
		return fmt.Errorf("invalid parameter: must not contain path separators")
	}
	if filepath.Clean(s) != s || strings.Contains(s, "..") {
		return fmt.Errorf("invalid parameter: must not contain path traversal sequences")
	}
	return nil
}

// EntityToMap converts an entity to a map for response serialization.
// Meta fields are flattened to the top level for template convenience,
// and also preserved as a nested "Meta" key for JSON consumers.
func EntityToMap(e *Entity) map[string]any {
	m := map[string]any{
		"ID":   e.ID,
		"Body": e.Body,
	}
	for k, v := range e.Meta {
		m[k] = v
	}
	m["Meta"] = e.Meta
	return m
}

// EntitiesToMaps converts a slice of entities to maps.
func EntitiesToMaps(entities []*Entity) []map[string]any {
	items := make([]map[string]any, 0, len(entities))
	for _, e := range entities {
		items = append(items, EntityToMap(e))
	}
	return items
}

// BodyToEntity converts a decoded request body (map) into an Entity.
// It extracts "id", "body", and "format" as top-level entity fields;
// remaining keys become Meta.
func BodyToEntity(body map[string]any) (*Entity, error) {
	e := &Entity{
		Meta:   make(map[string]any),
		Format: "json",
	}

	if id, ok := body["id"].(string); ok {
		e.ID = id
		delete(body, "id")
	}
	if b, ok := body["body"].(string); ok {
		e.Body = b
		delete(body, "body")
	}
	if f, ok := body["format"].(string); ok {
		e.Format = f
		delete(body, "format")
	}

	for k, v := range body {
		e.Meta[k] = v
	}
	return e, nil
}
