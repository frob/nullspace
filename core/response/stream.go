package response

import (
	"context"
	"io"
)

// StreamFormatter is an optional interface that formatters can implement
// to support incremental streaming. When streaming is active and the
// resolved formatter implements this interface, the pipeline calls these
// methods instead of Format().
type StreamFormatter interface {
	// StreamContentType returns the MIME type for streamed responses.
	StreamContentType() string

	// WriteStreamHeader writes optional preamble before items.
	WriteStreamHeader(ctx context.Context, w io.Writer, meta map[string]any) error

	// WriteStreamItem writes a single item to the writer.
	WriteStreamItem(ctx context.Context, w io.Writer, item any) error

	// WriteStreamFooter writes optional epilogue after all items.
	WriteStreamFooter(ctx context.Context, w io.Writer) error
}

// StreamIter is the interface used by the pipeline to pull items
// from a data source one at a time.
type StreamIter interface {
	// Next returns the next item, or (nil, nil) when exhausted.
	Next() (any, error)

	// Close releases resources. Must be called when iteration is complete.
	Close() error
}

// StreamResponse represents a streaming list response.
type StreamResponse struct {
	// Status is the HTTP status code. Defaults to 200 if not set.
	Status int

	// Headers are additional HTTP headers to set.
	Headers map[string]string

	// Meta holds stream metadata (e.g., Collection name, Total count).
	Meta map[string]any

	// Iter provides items one at a time.
	Iter StreamIter
}
