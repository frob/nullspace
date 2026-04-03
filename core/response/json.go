package response

import (
	"context"
	"encoding/json"
	"io"
)

// JSONFormatter serializes response data as JSON.
// It also implements StreamFormatter for NDJSON streaming.
type JSONFormatter struct{}

func (f *JSONFormatter) Name() string        { return "json" }
func (f *JSONFormatter) ContentType() string { return "application/json" }

func (f *JSONFormatter) Format(ctx context.Context, resp *Response) ([]byte, error) {
	// Check for pretty-print via query param.
	if r := HTTPRequestFromContext(ctx); r != nil {
		if r.URL.Query().Get("pretty") == "true" {
			return json.MarshalIndent(resp.Data, "", "  ")
		}
	}
	return json.Marshal(resp.Data)
}

// StreamContentType returns the NDJSON content type.
func (f *JSONFormatter) StreamContentType() string { return "application/x-ndjson" }

// WriteStreamHeader is a no-op for NDJSON.
func (f *JSONFormatter) WriteStreamHeader(_ context.Context, _ io.Writer, _ map[string]any) error {
	return nil
}

// WriteStreamItem writes a single JSON object followed by a newline.
func (f *JSONFormatter) WriteStreamItem(_ context.Context, w io.Writer, item any) error {
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

// WriteStreamFooter is a no-op for NDJSON.
func (f *JSONFormatter) WriteStreamFooter(_ context.Context, _ io.Writer) error {
	return nil
}

// Compile-time check.
var _ StreamFormatter = (*JSONFormatter)(nil)
