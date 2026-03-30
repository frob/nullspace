package response

import (
	"context"

	"github.com/frob/nullspace/kernel"
)

// FormatQueryParam resolves format from the ?format= query parameter.
// Useful for debugging and curl usage.
type FormatQueryParam struct{}

func NewFormatQueryParam() *FormatQueryParam { return &FormatQueryParam{} }

func (m *FormatQueryParam) Name() string { return "format.query_param" }
func (m *FormatQueryParam) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "format.query_param",
		DefaultEnabled: true,
	}
}

func (m *FormatQueryParam) Init(k *kernel.Kernel) error {
	k.HookResolve("response.format.resolve", 20, m.resolve)
	return nil
}

func (m *FormatQueryParam) Start(ctx context.Context) error { return nil }
func (m *FormatQueryParam) Stop(ctx context.Context) error  { return nil }

func (m *FormatQueryParam) resolve(ctx context.Context) (any, bool, error) {
	format := queryFormatFromContext(ctx)
	if format != "" {
		return format, true, nil
	}
	return nil, false, nil
}
