package response

import (
	"context"
	"strings"

	"github.com/frob/nullspace/kernel"
)

// FormatContentNegotiate resolves format from the Accept header.
type FormatContentNegotiate struct{}

func NewFormatContentNegotiate() *FormatContentNegotiate { return &FormatContentNegotiate{} }

func (m *FormatContentNegotiate) Name() string { return "format.content_negotiate" }
func (m *FormatContentNegotiate) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "format.content_negotiate",
		DefaultEnabled: true,
	}
}

func (m *FormatContentNegotiate) Init(k *kernel.Kernel) error {
	k.HookResolve("response.format.resolve", 30, m.resolve)
	return nil
}

func (m *FormatContentNegotiate) Start(ctx context.Context) error { return nil }
func (m *FormatContentNegotiate) Stop(ctx context.Context) error  { return nil }

func (m *FormatContentNegotiate) resolve(ctx context.Context) (any, bool, error) {
	accept := acceptHeaderFromContext(ctx)
	if accept == "" {
		return nil, false, nil
	}

	format := negotiateFormat(accept)
	if format != "" {
		return format, true, nil
	}
	return nil, false, nil
}

// negotiateFormat parses an Accept header and returns the best matching format.
// This is a simple implementation that checks for known MIME types.
func negotiateFormat(accept string) string {
	// Split on comma, check each media type.
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])

		switch mediaType {
		case "application/json":
			return "json"
		case "text/html":
			return "html"
		case "text/plain":
			return "text"
		}
	}

	// Check for wildcards.
	if strings.Contains(accept, "*/*") {
		return "" // don't resolve, let default handle it
	}

	return ""
}
