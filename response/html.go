package response

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"sync"
)

// HTMLFormatter renders response data using Go's html/template package.
type HTMLFormatter struct {
	dir       string
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// NewHTMLFormatter creates an HTML formatter that loads templates from the
// given directory. Templates are parsed lazily on first use.
func NewHTMLFormatter(dir string) *HTMLFormatter {
	return &HTMLFormatter{
		dir:       dir,
		templates: make(map[string]*template.Template),
	}
}

func (f *HTMLFormatter) Name() string        { return "html" }
func (f *HTMLFormatter) ContentType() string { return "text/html; charset=utf-8" }

func (f *HTMLFormatter) Format(_ context.Context, resp *Response) ([]byte, error) {
	if resp.Template == "" {
		return nil, fmt.Errorf("html formatter: no template specified")
	}

	tmpl, err := f.getTemplate(resp.Template)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, resp.Data); err != nil {
		return nil, fmt.Errorf("html formatter: execute %s: %w", resp.Template, err)
	}

	return buf.Bytes(), nil
}

func (f *HTMLFormatter) getTemplate(name string) (*template.Template, error) {
	f.mu.RLock()
	tmpl, ok := f.templates[name]
	f.mu.RUnlock()
	if ok {
		return tmpl, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if tmpl, ok := f.templates[name]; ok {
		return tmpl, nil
	}

	path := f.dir + "/" + name
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("html formatter: parse %s: %w", path, err)
	}

	f.templates[name] = tmpl
	return tmpl, nil
}
