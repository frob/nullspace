package session

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is a non-persistent, in-process session store.
// Sessions are lost on restart and are not shared across processes.
// Use for development and testing; prefer SQLStore for production.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*entry
	ttl     time.Duration
}

// NewMemoryStore creates an in-memory session store with the given TTL.
func NewMemoryStore(ttl time.Duration) *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]*entry),
		ttl:     ttl,
	}
}

func (s *MemoryStore) Create(ctx context.Context) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	sess := &Session{ID: id, Values: make(map[string]any)}
	s.mu.Lock()
	s.entries[id] = &entry{session: sess, expiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return sess, nil
}

func (s *MemoryStore) Load(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	e, ok := s.entries[id]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}
	if time.Now().After(e.expiresAt) {
		s.mu.Lock()
		delete(s.entries, id)
		s.mu.Unlock()
		return nil, nil
	}

	// Return a copy so callers cannot mutate the stored session directly.
	sess := &Session{
		ID:     e.session.ID,
		Values: make(map[string]any, len(e.session.Values)),
	}
	for k, v := range e.session.Values {
		sess.Values[k] = v
	}
	return sess, nil
}

func (s *MemoryStore) Save(ctx context.Context, sess *Session) error {
	s.mu.Lock()
	s.entries[sess.ID] = &entry{
		session:   sess,
		expiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	delete(s.entries, id)
	s.mu.Unlock()
	return nil
}
