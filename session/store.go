// Package session provides session management middleware for the nullspace framework.
//
// The session module registers three named middleware on the routing registry:
//   - session.load    — loads session from cookie if present; no enforcement
//   - session.require — enforces a valid session (401 or redirect if absent)
//   - session.ignore  — no-op; routes declare opt-out via session = "ignore" in TOML
//
// Usage in nullspace.toml:
//
//	[modules]
//	session = true
//
//	[session]
//	store  = "memory"   # or "sql"
//	cookie = "ns_session"
//	ttl    = "24h"
//	secure = true
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Session holds per-user data across requests.
type Session struct {
	ID     string
	Values map[string]any

	dirty bool
}

// Get retrieves a value from the session by key.
func (s *Session) Get(key string) (any, bool) {
	v, ok := s.Values[key]
	return v, ok
}

// Set stores a value and marks the session as modified.
func (s *Session) Set(key string, val any) {
	if s.Values == nil {
		s.Values = make(map[string]any)
	}
	s.Values[key] = val
	s.dirty = true
}

// Delete removes a key and marks the session as modified.
func (s *Session) Delete(key string) {
	delete(s.Values, key)
	s.dirty = true
}

// IsDirty reports whether the session has unsaved changes.
func (s *Session) IsDirty() bool {
	return s.dirty
}

// Store persists sessions. Implementations must be safe for concurrent use.
type Store interface {
	// Create creates a new session with a unique ID.
	Create(ctx context.Context) (*Session, error)
	// Load retrieves a session by ID. Returns (nil, nil) if not found or expired.
	Load(ctx context.Context, id string) (*Session, error)
	// Save persists a session, refreshing its TTL.
	Save(ctx context.Context, s *Session) error
	// Delete removes a session.
	Delete(ctx context.Context, id string) error
}

// entry wraps a Session with an expiry timestamp (used by MemoryStore).
type entry struct {
	session   *Session
	expiresAt time.Time
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}
