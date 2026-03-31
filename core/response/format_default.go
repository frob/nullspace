package response

import (
	"context"

	"github.com/frob/nullspace/kernel"
)

// FormatDefault always resolves to the kernel's configured default format.
// This is the lowest priority resolver — it ensures a format is always selected.
type FormatDefault struct {
	format string
}

func NewFormatDefault() *FormatDefault { return &FormatDefault{} }

func (m *FormatDefault) Name() string { return "format.default" }
func (m *FormatDefault) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "format.default",
		DefaultEnabled: true,
	}
}

func (m *FormatDefault) Init(k *kernel.Kernel) error {
	// Read the default format from the response pipeline config.
	var cfg PipelineConfig
	if err := k.Config().Decode("response", &cfg); err != nil {
		m.format = "json"
	} else {
		m.format = cfg.DefaultFormat
	}

	k.HookResolve("response.format.resolve", 40, m.resolve)
	return nil
}

func (m *FormatDefault) Start(ctx context.Context) error { return nil }
func (m *FormatDefault) Stop(ctx context.Context) error  { return nil }

func (m *FormatDefault) resolve(ctx context.Context) (any, bool, error) {
	return m.format, true, nil
}
