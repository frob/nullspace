package response

import (
	"context"

	"github.com/frob/nullspace/kernel"
)

// FormatRouteOverride resolves format from route metadata.
// Routes can declare a format via WithMeta("format", "json").
type FormatRouteOverride struct{}

func NewFormatRouteOverride() *FormatRouteOverride { return &FormatRouteOverride{} }

func (m *FormatRouteOverride) Name() string { return "format.route_override" }
func (m *FormatRouteOverride) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "format.route_override",
		DefaultEnabled: true,
	}
}

func (m *FormatRouteOverride) Init(k *kernel.Kernel) error {
	k.HookResolve("response.format.resolve", 10, m.resolve)
	return nil
}

func (m *FormatRouteOverride) Start(ctx context.Context) error { return nil }
func (m *FormatRouteOverride) Stop(ctx context.Context) error  { return nil }

func (m *FormatRouteOverride) resolve(ctx context.Context) (any, bool, error) {
	// Route metadata is stored in context by the request adapter.
	// We check for a "format" key in the route match metadata.
	// The request layer stores RouteMatch in context — we read it via
	// the route.meta context value if available.
	format := routeFormatFromContext(ctx)
	if format != "" {
		return format, true, nil
	}
	return nil, false, nil
}
