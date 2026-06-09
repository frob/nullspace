package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frob/nullspace/core/ipc"
	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/kernel"
)

// RegisterBridgeHandlers registers jobs.submit, jobs.list, jobs.cancel on the
// given TCP router. Exposed so tests can call it directly and so the
// kernel.after_init hook can opportunistically wire transports.
func RegisterBridgeHandlers(k *kernel.Kernel, router *tcp.Router, m *Module) {
	router.Handle("jobs.submit", makeSubmitHandler(m))
	router.Handle("jobs.list", makeListHandler(m))
	router.Handle("jobs.cancel", makeCancelHandler(m))
}

// hookBridgeRegistration wires RegisterBridgeHandlers onto every available
// transport (tcp, ipc) during kernel.after_init. Missing transports are
// silently skipped — bridge registration is opportunistic.
func hookBridgeRegistration(k *kernel.Kernel, m *Module) {
	k.Hook("kernel.after_init", 20, func(_ context.Context) error {
		if tcpAdapter, err := kernel.GetResource[*tcp.Adapter](k, "transport.tcp"); err == nil {
			RegisterBridgeHandlers(k, tcpAdapter.Router(), m)
			k.Logger().Info("jobs bridge registered on TCP")
		}
		if ipcAdapter, err := kernel.GetResource[*ipc.Adapter](k, "transport.ipc"); err == nil {
			RegisterBridgeHandlers(k, ipcAdapter.Router(), m)
			k.Logger().Info("jobs bridge registered on IPC")
		}
		return nil
	})
}

// ---- request/response types -------------------------------------------------

type submitRequest struct {
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Queue       string          `json:"queue"`
	MaxAttempts int             `json:"max_attempts"`
	RunAt       string          `json:"run_at"`
}

type listRequest struct {
	Statuses []JobStatus `json:"statuses"`
	Queue    string      `json:"queue"`
	Type     string      `json:"type"`
	Limit    int         `json:"limit"`
}

type cancelRequest struct {
	ID string `json:"id"`
}

// ---- helpers ----------------------------------------------------------------

func sendResult(conn *tcp.Conn, command string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return sendError(conn, command, 500, "marshal error: "+err.Error())
	}
	return conn.Send(command, b)
}

func sendError(conn *tcp.Conn, command string, status int, msg string) error {
	b, _ := json.Marshal(map[string]any{
		"error":  msg,
		"status": status,
	})
	return conn.Send(command, b)
}

// ---- handlers ---------------------------------------------------------------

func makeSubmitHandler(m *Module) tcp.HandlerFunc {
	return func(conn *tcp.Conn, command string, payload []byte) error {
		var req submitRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return sendError(conn, command, 400, "invalid payload: "+err.Error())
		}
		if req.Type == "" {
			return sendError(conn, command, 400, "missing type")
		}

		spec := JobSpec{
			Type:        req.Type,
			Payload:     req.Payload,
			Queue:       req.Queue,
			MaxAttempts: req.MaxAttempts,
		}
		if req.RunAt != "" {
			t, err := time.Parse(time.RFC3339, req.RunAt)
			if err != nil {
				return sendError(conn, command, 400, "invalid run_at: "+err.Error())
			}
			spec.RunAt = t
		}

		id, err := m.Submit(conn.Context(), spec)
		if err != nil {
			return sendError(conn, command, 500, err.Error())
		}

		return sendResult(conn, command, map[string]any{
			"id":     id,
			"status": "submitted",
		})
	}
}

func makeListHandler(m *Module) tcp.HandlerFunc {
	return func(conn *tcp.Conn, command string, payload []byte) error {
		var req listRequest
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &req); err != nil {
				return sendError(conn, command, 400, "invalid payload: "+err.Error())
			}
		}

		filter := ListFilter{
			Statuses: req.Statuses,
			Queue:    req.Queue,
			Type:     req.Type,
			Limit:    req.Limit,
		}

		items, err := m.Store().List(conn.Context(), filter)
		if err != nil {
			return sendError(conn, command, 500, err.Error())
		}

		return sendResult(conn, command, map[string]any{
			"Items": items,
			"Total": len(items),
		})
	}
}

func makeCancelHandler(m *Module) tcp.HandlerFunc {
	return func(conn *tcp.Conn, command string, payload []byte) error {
		var req cancelRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return sendError(conn, command, 400, "invalid payload: "+err.Error())
		}
		if req.ID == "" {
			return sendError(conn, command, 400, "missing id")
		}

		if err := m.Cancel(conn.Context(), req.ID); err != nil {
			return sendError(conn, command, 500, err.Error())
		}

		return sendResult(conn, command, map[string]any{
			"id":     req.ID,
			"status": "cancelled",
		})
	}
}
