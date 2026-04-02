// Package forms provides a configurable webform handler module.
//
// It demonstrates how to build a module that:
//   - Defines forms declaratively in TOML config
//   - Registers its own routes via the service locator
//   - Renders forms as HTML (GET) and handles submissions (POST)
//   - Parses both form-encoded and JSON request bodies
//   - Validates fields based on config rules
//   - Stores submissions via the file data module
//   - Fires custom hooks (form.before_submit, form.after_submit)
//     so other modules can add behavior (email, notifications, etc.)
//   - Uses the response pipeline for format-aware output
package forms

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/module/data/file"
)

//go:embed routes.toml
var routesData []byte

// Config holds the forms module's configuration.
type Config struct {
	// Forms maps form names to their definitions.
	Forms map[string]FormDef `json:"forms"`
}

// FormDef defines a single form.
type FormDef struct {
	// Title is displayed at the top of the rendered form.
	Title string `json:"title"`

	// Fields defines the form's input fields in order.
	Fields []FieldDef `json:"fields"`

	// SuccessMessage is shown after a successful submission.
	SuccessMessage string `json:"success_message"`

	// SuccessRedirect, if set, redirects after submission instead of
	// showing a message. Takes precedence over SuccessMessage.
	SuccessRedirect string `json:"success_redirect"`

	// Store is the collection name for saving submissions via the
	// file data module (e.g., "form-submissions"). If empty,
	// submissions are not persisted.
	Store string `json:"store"`
}

// FieldDef defines a single form field.
type FieldDef struct {
	// Name is the field's form name and storage key.
	Name string `json:"name"`

	// Label is displayed next to the input.
	Label string `json:"label"`

	// Type is the HTML input type (text, email, textarea, select, hidden).
	Type string `json:"type"`

	// Required marks the field as mandatory.
	Required bool `json:"required"`

	// Options holds select field options (for type "select").
	Options []string `json:"options"`
}

// Submission represents a stored form submission.
type Submission struct {
	Form      string         `json:"form"`
	Fields    map[string]any `json:"fields"`
	Timestamp string         `json:"timestamp"`
}

// Module provides configurable webform handling.
type Module struct {
	config   Config
	kernel   *kernel.Kernel
	fileMod  *file.Module
	pipeline *response.Pipeline
}

// New creates a new forms module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "forms" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "forms",
		Default:        Config{Forms: map[string]FormDef{}},
		DefaultEnabled: false, // opt-in
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	if err := k.Config().Decode("forms", &m.config); err != nil {
		m.config = Config{Forms: map[string]FormDef{}}
	}

	var err error
	m.pipeline, err = kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}

	// File module is optional — forms work without persistence.
	m.fileMod, _ = kernel.GetResource[*file.Module](k, "data.file")

	router, err := kernel.GetResource[*request.Router](k, "router")
	if err != nil {
		return err
	}

	// Register routes for each configured form.
	for name, def := range m.config.Forms {
		m.registerFormRoutes(router, name, def)
		k.Logger().Info("form registered", "name", name, "fields", len(def.Fields))
	}

	// Load this module's route definitions.
	if routingMod, err := kernel.GetResource[*routing.Module](k, "routing"); err == nil {
		if err := routingMod.LoadRoutes(routesData); err != nil {
			return err
		}
	}

	k.Provide("forms", m)
	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

func (m *Module) registerFormRoutes(router *request.Router, name string, def FormDef) {
	formName := name
	formDef := def

	// GET /forms/:name — render the form
	router.Get("/forms/"+formName, func(ctx *request.Context) error {
		return m.renderForm(ctx, formName, formDef, nil, nil)
	}, request.WithMeta("format", "html"))

	// POST /forms/:name — handle submission
	router.Post("/forms/"+formName, func(ctx *request.Context) error {
		return m.handleSubmit(ctx, formName, formDef)
	})

	// GET /api/forms/:name/submissions — list submissions (JSON)
	if formDef.Store != "" {
		router.Get("/api/forms/"+formName+"/submissions", func(ctx *request.Context) error {
			return m.listSubmissions(ctx, formDef)
		}, request.WithMeta("format", "json"))
	}
}

func (m *Module) renderForm(ctx *request.Context, name string, def FormDef, values map[string]any, errors map[string]string) error {
	data := map[string]any{
		"FormName": name,
		"Title":    def.Title,
		"Fields":   def.Fields,
		"Values":   values,
		"Errors":   errors,
		"Action":   "/forms/" + name,
	}
	resp := response.NewResponse(http.StatusOK, data).WithTemplate("form.html")
	return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
}

func (m *Module) handleSubmit(ctx *request.Context, name string, def FormDef) error {
	// Parse the request body.
	values, err := m.parseBody(ctx)
	if err != nil {
		resp := &response.Response{Status: http.StatusBadRequest, Error: err}
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}

	// Validate fields.
	errors := m.validate(def, values)
	if len(errors) > 0 {
		// Re-render form with errors (HTML) or return errors (JSON).
		if isJSONRequest(ctx) {
			resp := response.NewResponse(http.StatusUnprocessableEntity, map[string]any{
				"errors": errors,
			})
			return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
		}
		return m.renderForm(ctx, name, def, values, errors)
	}

	// Fire before_submit hook.
	submitCtx := withFormContext(ctx.Context(), name, values)
	if err := m.kernel.Fire("form.before_submit", submitCtx); err != nil {
		resp := &response.Response{Status: http.StatusUnprocessableEntity, Error: err}
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}

	// Store submission.
	if def.Store != "" && m.fileMod != nil {
		sub := &file.Entity{
			ID:     fmt.Sprintf("%s-%d", name, time.Now().UnixMilli()),
			Meta:   map[string]any{"form": name, "timestamp": time.Now().Format(time.RFC3339)},
			Body:   "",
			Format: "json",
		}
		// Merge field values into meta.
		for k, v := range values {
			sub.Meta[k] = v
		}
		if err := m.fileMod.Write(ctx.Context(), def.Store, sub.ID, sub); err != nil {
			ctx.Logger().Error("failed to store submission", "error", err)
		}
	}

	// Fire after_submit hook.
	_ = m.kernel.Fire("form.after_submit", submitCtx)

	// Respond with success.
	if isJSONRequest(ctx) {
		resp := response.NewResponse(http.StatusOK, map[string]string{
			"status":  "ok",
			"message": def.SuccessMessage,
		})
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}

	if def.SuccessRedirect != "" {
		http.Redirect(ctx.Writer, ctx.Request, def.SuccessRedirect, http.StatusSeeOther)
		return nil
	}

	data := map[string]any{
		"FormName": name,
		"Title":    def.Title,
		"Message":  def.SuccessMessage,
	}
	resp := response.NewResponse(http.StatusOK, data).WithTemplate("form_success.html")
	return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
}

func (m *Module) listSubmissions(ctx *request.Context, def FormDef) error {
	if m.fileMod == nil {
		resp := response.NewResponse(http.StatusOK, map[string]any{"submissions": []any{}})
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}

	entities, err := m.fileMod.List(ctx.Context(), def.Store)
	if err != nil {
		resp := &response.Response{Status: http.StatusInternalServerError, Error: err}
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}

	var submissions []map[string]any
	for _, e := range entities {
		submissions = append(submissions, e.Meta)
	}

	resp := response.NewResponse(http.StatusOK, map[string]any{"submissions": submissions})
	return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
}

func (m *Module) parseBody(ctx *request.Context) (map[string]any, error) {
	ct := ctx.Request.Header.Get("Content-Type")

	if strings.HasPrefix(ct, "application/json") {
		var values map[string]any
		if err := json.NewDecoder(ctx.Request.Body).Decode(&values); err != nil {
			return nil, fmt.Errorf("invalid JSON body: %w", err)
		}
		return values, nil
	}

	// Default: form-encoded.
	if err := ctx.Request.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form body: %w", err)
	}

	values := make(map[string]any)
	for key, vals := range ctx.Request.PostForm {
		if len(vals) == 1 {
			values[key] = vals[0]
		} else {
			values[key] = vals
		}
	}
	return values, nil
}

func (m *Module) validate(def FormDef, values map[string]any) map[string]string {
	errors := make(map[string]string)

	for _, field := range def.Fields {
		val, exists := values[field.Name]
		if field.Required {
			if !exists {
				errors[field.Name] = field.Label + " is required"
				continue
			}
			if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
				errors[field.Name] = field.Label + " is required"
			}
		}
	}

	return errors
}

func isJSONRequest(ctx *request.Context) bool {
	ct := ctx.Request.Header.Get("Content-Type")
	accept := ctx.Request.Header.Get("Accept")
	return strings.HasPrefix(ct, "application/json") ||
		strings.Contains(accept, "application/json")
}

// Context helpers for form hooks.

type formContextKey struct{ name string }

var (
	formNameKey   = formContextKey{"form.name"}
	formValuesKey = formContextKey{"form.values"}
)

func withFormContext(ctx context.Context, name string, values map[string]any) context.Context {
	ctx = context.WithValue(ctx, formNameKey, name)
	ctx = context.WithValue(ctx, formValuesKey, values)
	return ctx
}

// FormNameFromContext returns the form name from a hook context.
func FormNameFromContext(ctx context.Context) string {
	s, _ := ctx.Value(formNameKey).(string)
	return s
}

// FormValuesFromContext returns the submitted values from a hook context.
func FormValuesFromContext(ctx context.Context) map[string]any {
	m, _ := ctx.Value(formValuesKey).(map[string]any)
	return m
}
