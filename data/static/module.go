// Package static provides the static file serving module.
//
// It serves files from a configured directory as a fallback when no dynamic
// route matches. Files are served with appropriate Content-Type headers
// based on file extension.
package static

import (
	"context"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/request"
)

// Config holds the static file module's configuration.
type Config struct {
	Dir string `json:"dir" toml:"dir"`
}

// Module serves static files from a directory.
type Module struct {
	dir string
}

// New creates a new static file module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "data.static" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key: "data.static",
		Default: Config{
			Dir: "./public",
		},
		DefaultEnabled: true,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	var cfg Config
	if err := k.Config().Decode("data.static", &cfg); err != nil {
		cfg = Config{Dir: "./public"}
	}
	m.dir = cfg.Dir

	// Register as a fallback on the request adapter.
	adapter, err := kernel.GetResource[*request.Adapter](k, "request.adapter")
	if err != nil {
		return err
	}
	adapter.Fallback(m.serve)

	k.Provide("data.static", m)
	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

func (m *Module) Healthy(ctx context.Context) error {
	_, err := os.Stat(m.dir)
	return err
}

// serve handles a request by looking for a matching file in the static directory.
// Returns request.ErrNotHandled if no file exists at the requested path.
func (m *Module) serve(ctx *request.Context) error {
	// Clean the path to prevent directory traversal.
	urlPath := filepath.Clean(ctx.Request.URL.Path)
	if urlPath == "." {
		urlPath = "/"
	}

	// Build the filesystem path.
	fsPath := filepath.Join(m.dir, urlPath)

	// Ensure the resolved path is still within the static directory.
	absDir, _ := filepath.Abs(m.dir)
	absPath, _ := filepath.Abs(fsPath)
	if !strings.HasPrefix(absPath, absDir) {
		return request.ErrNotHandled
	}

	// Check if the file exists and is not a directory.
	info, err := os.Stat(fsPath)
	if err != nil || info.IsDir() {
		// Try index.html for directory paths.
		indexPath := filepath.Join(fsPath, "index.html")
		info, err = os.Stat(indexPath)
		if err != nil || info.IsDir() {
			return request.ErrNotHandled
		}
		fsPath = indexPath
	}

	// Read and serve the file.
	data, err := os.ReadFile(fsPath)
	if err != nil {
		return request.ErrNotHandled
	}

	// Set Content-Type from extension.
	ext := filepath.Ext(fsPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	ctx.Writer.Header().Set("Content-Type", contentType)
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Write(data)

	ctx.Logger().Debug("static file served", "file", fsPath)
	return nil
}
