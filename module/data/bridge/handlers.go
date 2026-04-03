package bridge

import (
	"encoding/json"
	"fmt"

	"github.com/frob/nullspace/core/tcp"
	"github.com/frob/nullspace/module/data/file"
)

// Request payload types.

type listRequest struct {
	Collection string `json:"collection"`
	Stream     bool   `json:"stream"`
}

type getRequest struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
}

type createRequest struct {
	Collection string         `json:"collection"`
	Body       map[string]any `json:"body"`
}

type updateRequest struct {
	Collection string         `json:"collection"`
	ID         string         `json:"id"`
	Body       map[string]any `json:"body"`
}

type deleteRequest struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
}

// sendResult marshals data as JSON and sends it back on the same command.
func sendResult(conn *tcp.Conn, command string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return sendError(conn, command, 500, "marshal error: "+err.Error())
	}
	return conn.Send(command, b)
}

// sendError sends a JSON error response on the same command.
func sendError(conn *tcp.Conn, command string, status int, msg string) error {
	b, _ := json.Marshal(map[string]any{
		"error":  msg,
		"status": status,
	})
	return conn.Send(command, b)
}

func (m *Module) handleList(conn *tcp.Conn, command string, payload []byte) error {
	var req listRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return sendError(conn, command, 400, "invalid payload: "+err.Error())
	}
	if req.Collection == "" {
		return sendError(conn, command, 400, "missing collection")
	}
	if err := file.ValidateSegment(req.Collection); err != nil {
		return sendError(conn, command, 400, err.Error())
	}

	if req.Stream {
		return m.handleListStream(conn, req)
	}

	entities, err := m.fileMod.List(conn.Context(), req.Collection)
	if err != nil {
		return sendError(conn, command, 500, "internal error")
	}

	items := file.EntitiesToMaps(entities)
	return sendResult(conn, command, map[string]any{
		"Items":      items,
		"Collection": req.Collection,
	})
}

func (m *Module) handleListStream(conn *tcp.Conn, req listRequest) error {
	iter, err := m.fileMod.ListIter(conn.Context(), req.Collection)
	if err != nil {
		return sendError(conn, "data.list", 500, "internal error")
	}

	total := 0
	if iter != nil {
		total = iter.Total()
		defer iter.Close()
	}

	// Send start envelope.
	if err := sendResult(conn, "data.list.start", map[string]any{
		"collection": req.Collection,
		"total":      total,
	}); err != nil {
		return err
	}

	// Stream items.
	count := 0
	if iter != nil {
		for {
			entity, err := iter.Next()
			if err != nil {
				return err
			}
			if entity == nil {
				break
			}
			if err := sendResult(conn, "data.list.item", file.EntityToMap(entity)); err != nil {
				return err
			}
			count++
		}
	}

	// Send end envelope.
	return sendResult(conn, "data.list.end", map[string]any{
		"collection": req.Collection,
		"count":      count,
	})
}

func (m *Module) handleGet(conn *tcp.Conn, command string, payload []byte) error {
	var req getRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return sendError(conn, command, 400, "invalid payload: "+err.Error())
	}
	if req.Collection == "" {
		return sendError(conn, command, 400, "missing collection")
	}
	if req.ID == "" {
		return sendError(conn, command, 400, "missing id")
	}
	if err := file.ValidateSegment(req.Collection); err != nil {
		return sendError(conn, command, 400, err.Error())
	}
	if err := file.ValidateSegment(req.ID); err != nil {
		return sendError(conn, command, 400, err.Error())
	}

	entity, err := m.fileMod.Read(conn.Context(), req.Collection, req.ID)
	if err != nil {
		return sendError(conn, command, 404, fmt.Sprintf("not found: %s/%s", req.Collection, req.ID))
	}

	return sendResult(conn, command, file.EntityToMap(entity))
}

func (m *Module) handleCreate(conn *tcp.Conn, command string, payload []byte) error {
	var req createRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return sendError(conn, command, 400, "invalid payload: "+err.Error())
	}
	if req.Collection == "" {
		return sendError(conn, command, 400, "missing collection")
	}
	if req.Body == nil {
		return sendError(conn, command, 400, "missing body")
	}
	if err := file.ValidateSegment(req.Collection); err != nil {
		return sendError(conn, command, 400, err.Error())
	}

	entity, err := file.BodyToEntity(req.Body)
	if err != nil {
		return sendError(conn, command, 400, err.Error())
	}
	if entity.ID == "" {
		return sendError(conn, command, 400, "missing 'id' field in body")
	}
	if err := file.ValidateSegment(entity.ID); err != nil {
		return sendError(conn, command, 400, err.Error())
	}

	if err := m.fileMod.Write(conn.Context(), req.Collection, entity.ID, entity); err != nil {
		return sendError(conn, command, 500, "internal error")
	}

	return sendResult(conn, command, map[string]any{
		"id":     entity.ID,
		"status": "created",
	})
}

func (m *Module) handleUpdate(conn *tcp.Conn, command string, payload []byte) error {
	var req updateRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return sendError(conn, command, 400, "invalid payload: "+err.Error())
	}
	if req.Collection == "" {
		return sendError(conn, command, 400, "missing collection")
	}
	if req.ID == "" {
		return sendError(conn, command, 400, "missing id")
	}
	if req.Body == nil {
		return sendError(conn, command, 400, "missing body")
	}
	if err := file.ValidateSegment(req.Collection); err != nil {
		return sendError(conn, command, 400, err.Error())
	}
	if err := file.ValidateSegment(req.ID); err != nil {
		return sendError(conn, command, 400, err.Error())
	}

	entity, err := file.BodyToEntity(req.Body)
	if err != nil {
		return sendError(conn, command, 400, err.Error())
	}
	entity.ID = req.ID

	if err := m.fileMod.Write(conn.Context(), req.Collection, req.ID, entity); err != nil {
		return sendError(conn, command, 500, "internal error")
	}

	return sendResult(conn, command, map[string]any{
		"id":     req.ID,
		"status": "updated",
	})
}

func (m *Module) handleDelete(conn *tcp.Conn, command string, payload []byte) error {
	var req deleteRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return sendError(conn, command, 400, "invalid payload: "+err.Error())
	}
	if req.Collection == "" {
		return sendError(conn, command, 400, "missing collection")
	}
	if req.ID == "" {
		return sendError(conn, command, 400, "missing id")
	}
	if err := file.ValidateSegment(req.Collection); err != nil {
		return sendError(conn, command, 400, err.Error())
	}
	if err := file.ValidateSegment(req.ID); err != nil {
		return sendError(conn, command, 400, err.Error())
	}

	if err := m.fileMod.Delete(conn.Context(), req.Collection, req.ID); err != nil {
		return sendError(conn, command, 404, fmt.Sprintf("not found: %s/%s", req.Collection, req.ID))
	}

	return sendResult(conn, command, map[string]any{
		"id":     req.ID,
		"status": "deleted",
	})
}
