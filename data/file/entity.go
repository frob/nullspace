// Package file provides file-based entity storage.
//
// Entities are stored as files on disk — one file per record. Supported
// formats are markdown (with YAML or TOML frontmatter), JSON, and TOML.
// Directory structure maps to entity type: content/posts/my-post.md
package file

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
