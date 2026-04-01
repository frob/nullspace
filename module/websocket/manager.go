package websocket

import (
	"sync"

	"nhooyr.io/websocket"
)

// Manager tracks active WebSocket connections and provides room-based
// broadcast. It is safe for concurrent use and available to other modules
// via the service locator at key "websocket.manager".
type Manager struct {
	mu    sync.RWMutex
	conns map[string]*Conn            // id -> conn
	rooms map[string]map[string]*Conn // room -> id -> conn
}

// NewManager creates an empty connection manager.
func NewManager() *Manager {
	return &Manager{
		conns: make(map[string]*Conn),
		rooms: make(map[string]map[string]*Conn),
	}
}

// Add registers a connection with the manager.
func (m *Manager) Add(c *Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[c.ID] = c
}

// Remove unregisters a connection and leaves all rooms.
func (m *Manager) Remove(c *Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, c.ID)
	for room, members := range m.rooms {
		delete(members, c.ID)
		if len(members) == 0 {
			delete(m.rooms, room)
		}
	}
}

// Join adds a connection to a named room.
func (m *Manager) Join(c *Conn, room string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[room] == nil {
		m.rooms[room] = make(map[string]*Conn)
	}
	m.rooms[room][c.ID] = c
}

// Leave removes a connection from a named room.
func (m *Manager) Leave(c *Conn, room string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if members, ok := m.rooms[room]; ok {
		delete(members, c.ID)
		if len(members) == 0 {
			delete(m.rooms, room)
		}
	}
}

// Broadcast sends a message to all connected clients. Connections that
// fail to receive the message are skipped (not removed).
func (m *Manager) Broadcast(msg Message) {
	m.mu.RLock()
	targets := make([]*Conn, 0, len(m.conns))
	for _, c := range m.conns {
		targets = append(targets, c)
	}
	m.mu.RUnlock()

	for _, c := range targets {
		_ = c.Send(msg)
	}
}

// BroadcastTo sends a message to all connections in a room.
func (m *Manager) BroadcastTo(room string, msg Message) {
	m.mu.RLock()
	members := m.rooms[room]
	targets := make([]*Conn, 0, len(members))
	for _, c := range members {
		targets = append(targets, c)
	}
	m.mu.RUnlock()

	for _, c := range targets {
		_ = c.Send(msg)
	}
}

// Count returns the number of active connections.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}

// RoomCount returns the number of connections in a room.
func (m *Manager) RoomCount(room string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms[room])
}

// CloseAll gracefully closes every active connection.
func (m *Manager) CloseAll(code websocket.StatusCode, reason string) {
	m.mu.Lock()
	conns := make([]*Conn, 0, len(m.conns))
	for _, c := range m.conns {
		conns = append(conns, c)
	}
	m.conns = make(map[string]*Conn)
	m.rooms = make(map[string]map[string]*Conn)
	m.mu.Unlock()

	for _, c := range conns {
		_ = c.Close(code, reason)
	}
}
