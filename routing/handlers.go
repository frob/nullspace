package routing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/frob/nullspace/data/file"
	"github.com/frob/nullspace/kernel"
	"github.com/frob/nullspace/request"
	"github.com/frob/nullspace/response"
)

// builtinPrefix is the naming convention for built-in handlers.
const builtinPrefix = "data."

// deps holds shared dependencies for built-in handlers.
type deps struct {
	kernel   *kernel.Kernel
	pipeline *response.Pipeline
	fileMod  *file.Module
}

// registerBuiltins registers all built-in handlers on the registry.
func registerBuiltins(reg *Registry, d *deps) {
	reg.HandleFunc("data.list", makeDataList(d))
	reg.HandleFunc("data.get", makeDataGet(d))
	reg.HandleFunc("data.create", makeDataCreate(d))
	reg.HandleFunc("data.update", makeDataUpdate(d))
	reg.HandleFunc("data.delete", makeDataDelete(d))
	reg.HandleFunc("template", makeTemplate(d))
	reg.HandleFunc("redirect", nil) // placeholder — redirect is handled specially
}

// isBuiltin returns true if the handler name is a built-in.
func isBuiltin(name string) bool {
	switch name {
	case "data.list", "data.get", "data.create", "data.update", "data.delete",
		"template", "redirect":
		return true
	}
	return false
}

// makeDataList returns a handler that lists entities from a collection.
// Route config must set `collection`. Template is optional for HTML.
func makeDataList(d *deps) request.HandlerFunc {
	return func(ctx *request.Context) error {
		route := ctx.Route()
		collection := route.Meta["_collection"]

		entities, err := d.fileMod.List(ctx.Context(), collection)
		if err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusInternalServerError, Error: err})
		}

		items := entitiesToMaps(entities)

		data := map[string]any{
			"Items":      items,
			"Collection": collection,
		}

		resp := response.NewResponse(http.StatusOK, data)
		if tmpl := route.Meta["_template"]; tmpl != "" {
			resp.Template = tmpl
		}
		return d.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}
}

// makeDataGet returns a handler that fetches a single entity by ID.
func makeDataGet(d *deps) request.HandlerFunc {
	return func(ctx *request.Context) error {
		route := ctx.Route()
		collection := route.Meta["_collection"]
		paramName := route.Meta["_data_param"]
		if paramName == "" {
			paramName = "id"
		}

		id := ctx.Param(paramName)
		if id == "" {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusBadRequest,
					Error: fmt.Errorf("missing parameter: %s", paramName)})
		}

		entity, err := d.fileMod.Read(ctx.Context(), collection, id)
		if err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusNotFound,
					Error: fmt.Errorf("not found: %s/%s", collection, id)})
		}

		data := entityToMap(entity)

		resp := response.NewResponse(http.StatusOK, data)
		if tmpl := route.Meta["_template"]; tmpl != "" {
			resp.Template = tmpl
		}
		return d.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}
}

// makeDataCreate returns a handler that creates a new entity from the request body.
func makeDataCreate(d *deps) request.HandlerFunc {
	return func(ctx *request.Context) error {
		route := ctx.Route()
		collection := route.Meta["_collection"]

		entity, err := entityFromBody(ctx)
		if err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusBadRequest, Error: err})
		}

		if entity.ID == "" {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusBadRequest,
					Error: fmt.Errorf("missing 'id' field")})
		}

		if err := d.fileMod.Write(ctx.Context(), collection, entity.ID, entity); err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusInternalServerError, Error: err})
		}

		resp := response.NewResponse(http.StatusCreated, map[string]any{
			"id":     entity.ID,
			"status": "created",
		})
		return d.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}
}

// makeDataUpdate returns a handler that updates an existing entity.
func makeDataUpdate(d *deps) request.HandlerFunc {
	return func(ctx *request.Context) error {
		route := ctx.Route()
		collection := route.Meta["_collection"]
		paramName := route.Meta["_data_param"]
		if paramName == "" {
			paramName = "id"
		}

		id := ctx.Param(paramName)
		if id == "" {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusBadRequest,
					Error: fmt.Errorf("missing parameter: %s", paramName)})
		}

		entity, err := entityFromBody(ctx)
		if err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusBadRequest, Error: err})
		}
		entity.ID = id

		if err := d.fileMod.Write(ctx.Context(), collection, id, entity); err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusInternalServerError, Error: err})
		}

		resp := response.NewResponse(http.StatusOK, map[string]any{
			"id":     id,
			"status": "updated",
		})
		return d.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}
}

// makeDataDelete returns a handler that deletes an entity.
func makeDataDelete(d *deps) request.HandlerFunc {
	return func(ctx *request.Context) error {
		route := ctx.Route()
		collection := route.Meta["_collection"]
		paramName := route.Meta["_data_param"]
		if paramName == "" {
			paramName = "id"
		}

		id := ctx.Param(paramName)
		if id == "" {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusBadRequest,
					Error: fmt.Errorf("missing parameter: %s", paramName)})
		}

		if err := d.fileMod.Delete(ctx.Context(), collection, id); err != nil {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusNotFound,
					Error: fmt.Errorf("not found: %s/%s", collection, id)})
		}

		resp := response.NewResponse(http.StatusOK, map[string]any{
			"id":     id,
			"status": "deleted",
		})
		return d.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}
}

// makeTemplate returns a handler that renders a template with no data fetching.
func makeTemplate(d *deps) request.HandlerFunc {
	return func(ctx *request.Context) error {
		route := ctx.Route()
		tmpl := route.Meta["_template"]
		if tmpl == "" {
			return d.pipeline.Write(ctx.Context(), ctx.Writer,
				&response.Response{Status: http.StatusInternalServerError,
					Error: fmt.Errorf("no template specified for route")})
		}

		data := map[string]any{
			"Path":   ctx.Request.URL.Path,
			"Params": route.Params,
		}

		resp := response.NewResponse(http.StatusOK, data).WithTemplate(tmpl)
		return d.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	}
}

// makeRedirectHandler creates a handler for a specific redirect target.
func makeRedirectHandler(target string, statusCode int) request.HandlerFunc {
	if statusCode == 0 {
		statusCode = http.StatusSeeOther
	}
	return func(ctx *request.Context) error {
		http.Redirect(ctx.Writer, ctx.Request, target, statusCode)
		return nil
	}
}

// dataInjectionMiddleware wraps a handler to pre-load data into context state.
func dataInjectionMiddleware(d *deps, collection, paramName string) request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if paramName != "" {
				// Single entity lookup.
				id := ctx.Param(paramName)
				if id != "" {
					entity, err := d.fileMod.Read(ctx.Context(), collection, id)
					if err == nil {
						ctx.SetState("data.entity", entity)
						ctx.SetState("data.entity.map", entityToMap(entity))
					}
				}
			} else {
				// List lookup.
				entities, err := d.fileMod.List(ctx.Context(), collection)
				if err == nil {
					ctx.SetState("data.entities", entities)
					ctx.SetState("data.entities.maps", entitiesToMaps(entities))
				}
			}
			return next(ctx)
		}
	}
}

// Helper functions.

func entityToMap(e *file.Entity) map[string]any {
	m := map[string]any{
		"ID":   e.ID,
		"Body": e.Body,
	}
	// Flatten meta into the top level for template convenience.
	for k, v := range e.Meta {
		m[k] = v
	}
	// Also keep meta as a nested map for JSON.
	m["Meta"] = e.Meta
	return m
}

func entitiesToMaps(entities []*file.Entity) []map[string]any {
	items := make([]map[string]any, 0, len(entities))
	for _, e := range entities {
		items = append(items, entityToMap(e))
	}
	return items
}

func entityFromBody(ctx *request.Context) (*file.Entity, error) {
	ct := ctx.Request.Header.Get("Content-Type")

	if strings.HasPrefix(ct, "application/json") {
		var body map[string]any
		if err := json.NewDecoder(ctx.Request.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return bodyToEntity(body)
	}

	// Form-encoded.
	if err := ctx.Request.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form: %w", err)
	}
	body := make(map[string]any)
	for key, vals := range ctx.Request.PostForm {
		if len(vals) == 1 {
			body[key] = vals[0]
		} else {
			body[key] = vals
		}
	}
	return bodyToEntity(body)
}

func bodyToEntity(body map[string]any) (*file.Entity, error) {
	e := &file.Entity{
		Meta:   make(map[string]any),
		Format: "json",
	}

	if id, ok := body["id"].(string); ok {
		e.ID = id
		delete(body, "id")
	}
	if b, ok := body["body"].(string); ok {
		e.Body = b
		delete(body, "body")
	}
	if f, ok := body["format"].(string); ok {
		e.Format = f
		delete(body, "format")
	}

	for k, v := range body {
		e.Meta[k] = v
	}
	return e, nil
}
