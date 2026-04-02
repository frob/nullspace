// Package chat demonstrates the WebSocket module with a simple chat room.
//
// It registers a WebSocket handler that broadcasts messages to all connected
// clients. A username can be set via the "name" query parameter on the
// initial connection URL (e.g., /ws/chat?name=Alice).
//
// Enable in nullspace.toml:
//
//	[modules]
//	websocket = true
//	chat = true
package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frob/nullspace/core/request"
	"github.com/frob/nullspace/core/response"
	"github.com/frob/nullspace/core/routing"
	"github.com/frob/nullspace/kernel"
	ws "github.com/frob/nullspace/module/websocket"
)

// chatMessage is the JSON envelope for chat messages.
type chatMessage struct {
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"` // "message", "join", "leave"
}

// Module provides a chat room backed by the websocket module.
type Module struct {
	kernel   *kernel.Kernel
	wsMod    *ws.Module
	pipeline *response.Pipeline
}

// New creates a new chat module.
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string { return "chat" }

func (m *Module) Config() kernel.ModuleConfig {
	return kernel.ModuleConfig{
		Key:            "chat",
		Default:        struct{}{},
		DefaultEnabled: false,
	}
}

func (m *Module) Init(k *kernel.Kernel) error {
	m.kernel = k

	wsMod, err := kernel.GetResource[*ws.Module](k, "websocket")
	if err != nil {
		return err
	}
	m.wsMod = wsMod

	m.pipeline, err = kernel.GetResource[*response.Pipeline](k, "response.pipeline")
	if err != nil {
		return err
	}

	reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
	if err != nil {
		return err
	}

	// Register middleware that reads the "name" query param.
	reg.Middleware("chat.name", m.nameMiddleware())

	// Register the WebSocket chat handler.
	mgr := wsMod.Manager()

	wsMod.HandleFunc("chat", func(conn *ws.Conn, msg ws.Message) error {
		user := "anonymous"
		if u, ok := conn.State("chat.user"); ok {
			if s, ok := u.(string); ok && s != "" {
				user = s
			}
		}

		out, _ := json.Marshal(chatMessage{
			User:      user,
			Text:      string(msg.Data),
			Timestamp: time.Now().Format(time.RFC3339),
			Type:      "message",
		})

		mgr.BroadcastTo("chat", ws.TextMessage(string(out)))
		return nil
	})

	// Register the HTML page handler for the chat UI.
	reg.HandleFunc("chat.page", func(ctx *request.Context) error {
		resp := response.NewResponse(200, map[string]any{
			"Title": "Chat",
		})
		resp.Template = "chat.html"
		return m.pipeline.Write(ctx.Context(), ctx.Writer, resp)
	})

	k.Logger().Info("chat module initialized")
	return nil
}

func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

// nameMiddleware reads the "name" query parameter and stores it in request
// state so the WebSocket connection inherits it.
func (m *Module) nameMiddleware() request.Middleware {
	return func(next request.HandlerFunc) request.HandlerFunc {
		return func(ctx *request.Context) error {
			if name := ctx.Request.URL.Query().Get("name"); name != "" {
				ctx.SetState("chat.user", name)
			}
			return next(ctx)
		}
	}
}
