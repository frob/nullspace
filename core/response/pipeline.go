package response

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frob/nullspace/kernel"
)

// PipelineConfig holds the response pipeline's configuration.
type PipelineConfig struct {
	DefaultFormat string `json:"default_format" toml:"default_format"`
	TemplateDir   string `json:"template_dir" toml:"template_dir"`
}

// Pipeline is the response module. It resolves the response format via the
// hook bus, selects the appropriate formatter, and writes the serialized
// response to the client.
type Pipeline struct {
	kernel     *kernel.Kernel
	formatters map[string]Formatter
	defaultFmt string
}

// NewPipeline creates a new response pipeline module.
func NewPipeline() *Pipeline {
	return &Pipeline{
		formatters: make(map[string]Formatter),
	}
}

func (p *Pipeline) Name() string { return "response" }

func (p *Pipeline) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "response",
		Default: PipelineConfig{
			DefaultFormat: "json",
			TemplateDir:   "./templates",
		},
		DefaultEnabled: true,
	}
}

func (p *Pipeline) Init(k *kernel.Kernel) error {
	p.kernel = k

	var cfg PipelineConfig
	if err := k.Config().Decode("response", &cfg); err != nil {
		cfg = PipelineConfig{DefaultFormat: "json", TemplateDir: "./templates"}
	}
	p.defaultFmt = cfg.DefaultFormat

	// Register built-in formatters.
	p.RegisterFormatter(&JSONFormatter{})
	p.RegisterFormatter(NewHTMLFormatter(cfg.TemplateDir))
	p.RegisterFormatter(&TextFormatter{})
	p.RegisterFormatter(&ANSIFormatter{})

	k.Provide("response.pipeline", p)
	return nil
}

func (p *Pipeline) Start(ctx context.Context) error { return nil }
func (p *Pipeline) Stop(ctx context.Context) error  { return nil }

// RegisterFormatter adds a formatter to the pipeline.
func (p *Pipeline) RegisterFormatter(f Formatter) {
	p.formatters[f.Name()] = f
	if p.kernel != nil {
		p.kernel.Logger().Debug("formatter registered", "name", f.Name())
	}
}

// Write resolves the format, serializes the response, and writes it to w.
// The ctx should carry the config snapshot, HTTP request, and format context values.
func (p *Pipeline) Write(ctx context.Context, w http.ResponseWriter, resp *Response) error {
	// Handle error responses.
	if resp.Error != nil {
		return p.writeError(ctx, w, resp)
	}

	// Fire response.before_write hooks.
	_ = p.kernel.Fire("response.before_write", ctx)

	// Resolve format.
	format, err := p.resolveFormat(ctx)
	if err != nil {
		return err
	}

	// Find formatter.
	formatter, ok := p.formatters[format]
	if !ok {
		return fmt.Errorf("no formatter for format %q", format)
	}

	// Serialize.
	data, err := formatter.Format(ctx, resp)
	if err != nil {
		return err
	}

	// Write response.
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", formatter.ContentType())
	w.WriteHeader(status)
	w.Write(data)

	// Fire response.after_write hooks.
	_ = p.kernel.Fire("response.after_write", ctx)

	return nil
}

// resolveFormat uses the hook bus to determine the response format.
// Falls back to the configured default if no resolver handles it.
func (p *Pipeline) resolveFormat(ctx context.Context) (string, error) {
	val, err := p.kernel.Resolve("response.format.resolve", ctx)
	if err != nil {
		return "", fmt.Errorf("format resolution: %w", err)
	}
	if val != nil {
		if format, ok := val.(string); ok && format != "" {
			return format, nil
		}
	}
	return p.defaultFmt, nil
}

// WriteStream writes a streaming response. It resolves the format and checks
// if the formatter implements StreamFormatter. If not, it falls back to
// draining the iterator and using the normal batch Write path.
func (p *Pipeline) WriteStream(ctx context.Context, w http.ResponseWriter, sr *StreamResponse) error {
	_ = p.kernel.Fire("response.before_write", ctx)

	format, err := p.resolveFormat(ctx)
	if err != nil {
		return err
	}

	formatter, ok := p.formatters[format]
	if !ok {
		return fmt.Errorf("no formatter for format %q", format)
	}

	sf, ok := formatter.(StreamFormatter)
	if !ok {
		return p.writeStreamAsBatch(ctx, w, sr, formatter)
	}

	status := sr.Status
	if status == 0 {
		status = http.StatusOK
	}

	for k, v := range sr.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", sf.StreamContentType())
	w.WriteHeader(status)

	if err := sf.WriteStreamHeader(ctx, w, sr.Meta); err != nil {
		return err
	}
	flushWriter(w)

	for {
		item, err := sr.Iter.Next()
		if err != nil {
			return err
		}
		if item == nil {
			break
		}
		if err := sf.WriteStreamItem(ctx, w, item); err != nil {
			return err
		}
		flushWriter(w)
	}

	if err := sf.WriteStreamFooter(ctx, w); err != nil {
		return err
	}
	flushWriter(w)

	_ = p.kernel.Fire("response.after_write", ctx)
	return nil
}

// writeStreamAsBatch drains the iterator into a slice and uses the normal
// batch Write path. Used when the resolved formatter does not implement
// StreamFormatter.
func (p *Pipeline) writeStreamAsBatch(ctx context.Context, w http.ResponseWriter, sr *StreamResponse, formatter Formatter) error {
	defer sr.Iter.Close()

	var items []any
	for {
		item, err := sr.Iter.Next()
		if err != nil {
			return err
		}
		if item == nil {
			break
		}
		items = append(items, item)
	}

	collection, _ := sr.Meta["Collection"].(string)
	resp := &Response{
		Status:  sr.Status,
		Headers: sr.Headers,
		Data:    map[string]any{"Items": items, "Collection": collection},
	}
	return p.Write(ctx, w, resp)
}

func flushWriter(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (p *Pipeline) writeError(ctx context.Context, w http.ResponseWriter, resp *Response) error {
	_ = p.kernel.Fire("request.error", ctx)

	status := resp.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}

	// Try to use the resolved format for the error response.
	format, _ := p.resolveFormat(ctx)
	formatter, ok := p.formatters[format]
	if !ok {
		formatter = &JSONFormatter{}
	}

	errData := map[string]any{
		"error":  resp.Error.Error(),
		"status": status,
	}
	errResp := &Response{Status: status, Data: errData}

	data, err := formatter.Format(ctx, errResp)
	if err != nil {
		// Last resort: plain text.
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		fmt.Fprintf(w, "%d %s", status, resp.Error.Error())
		return nil
	}

	w.Header().Set("Content-Type", formatter.ContentType())
	w.WriteHeader(status)
	w.Write(data)
	return nil
}
