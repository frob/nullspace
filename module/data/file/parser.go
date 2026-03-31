package file

import (
	"encoding/json"
	"fmt"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// parseFile parses file content into an Entity based on the file extension.
func parseFile(id, ext string, data []byte) (*Entity, error) {
	switch ext {
	case ".md", ".markdown":
		return parseMarkdown(id, data)
	case ".json":
		return parseJSON(id, data)
	case ".toml":
		return parseTOML(id, data)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// parseMarkdown parses a markdown file with optional frontmatter.
// Supports YAML frontmatter (--- delimiters) and TOML frontmatter (+++ delimiters).
func parseMarkdown(id string, data []byte) (*Entity, error) {
	content := string(data)
	meta := make(map[string]any)
	body := content

	if strings.HasPrefix(content, "---\n") {
		// YAML frontmatter.
		end := strings.Index(content[4:], "\n---")
		if end >= 0 {
			fmData := content[4 : 4+end]
			body = strings.TrimLeft(content[4+end+4:], "\n")
			if err := yaml.Unmarshal([]byte(fmData), &meta); err != nil {
				return nil, fmt.Errorf("parse YAML frontmatter: %w", err)
			}
		}
	} else if strings.HasPrefix(content, "+++\n") {
		// TOML frontmatter.
		end := strings.Index(content[4:], "\n+++")
		if end >= 0 {
			fmData := content[4 : 4+end]
			body = strings.TrimLeft(content[4+end+4:], "\n")
			if err := toml.Unmarshal([]byte(fmData), &meta); err != nil {
				return nil, fmt.Errorf("parse TOML frontmatter: %w", err)
			}
		}
	}

	return &Entity{
		ID:     id,
		Meta:   meta,
		Body:   body,
		Format: "markdown",
	}, nil
}

// parseJSON parses a JSON file into an Entity.
// The "body" key (if present) is extracted as the entity body.
func parseJSON(id string, data []byte) (*Entity, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var body string
	if b, ok := raw["body"].(string); ok {
		body = b
		delete(raw, "body")
	}

	return &Entity{
		ID:     id,
		Meta:   raw,
		Body:   body,
		Format: "json",
	}, nil
}

// parseTOML parses a TOML file into an Entity.
// The "body" key (if present) is extracted as the entity body.
func parseTOML(id string, data []byte) (*Entity, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}

	var body string
	if b, ok := raw["body"].(string); ok {
		body = b
		delete(raw, "body")
	}

	return &Entity{
		ID:     id,
		Meta:   raw,
		Body:   body,
		Format: "toml",
	}, nil
}

// serializeEntity converts an Entity back to bytes for writing.
func serializeEntity(e *Entity, format string) ([]byte, error) {
	switch format {
	case "markdown":
		return serializeMarkdown(e)
	case "json":
		return serializeJSON(e)
	case "toml":
		return serializeTOML(e)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func serializeMarkdown(e *Entity) ([]byte, error) {
	var buf strings.Builder

	if len(e.Meta) > 0 {
		fmBytes, err := yaml.Marshal(e.Meta)
		if err != nil {
			return nil, fmt.Errorf("serialize YAML frontmatter: %w", err)
		}
		buf.WriteString("---\n")
		buf.Write(fmBytes)
		buf.WriteString("---\n\n")
	}

	buf.WriteString(e.Body)
	return []byte(buf.String()), nil
}

func serializeJSON(e *Entity) ([]byte, error) {
	data := make(map[string]any, len(e.Meta)+1)
	for k, v := range e.Meta {
		data[k] = v
	}
	if e.Body != "" {
		data["body"] = e.Body
	}

	return json.MarshalIndent(data, "", "  ")
}

func serializeTOML(e *Entity) ([]byte, error) {
	data := make(map[string]any, len(e.Meta)+1)
	for k, v := range e.Meta {
		data[k] = v
	}
	if e.Body != "" {
		data["body"] = e.Body
	}

	return toml.Marshal(data)
}
