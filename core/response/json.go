package response

import (
	"context"
	"encoding/json"
)

// JSONFormatter serializes response data as JSON.
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
