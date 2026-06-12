package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SQLStore is a SQL-backed session store. It creates the sessions table on
// first use and survives process restarts. Requires the data.sql module.
type SQLStore struct {
	db  *sql.DB
	ttl time.Duration
}

// NewSQLStore creates a SQL-backed store. The sessions table is created by
// the migration registry during kernel.after_init — do not call this before
// migrations have run.
func NewSQLStore(db *sql.DB, ttl time.Duration) *SQLStore {
	return &SQLStore{db: db, ttl: ttl}
}

func (s *SQLStore) Create(ctx context.Context) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	sess := &Session{ID: id, Values: make(map[string]any)}
	if err := s.Save(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *SQLStore) Load(ctx context.Context, id string) (*Session, error) {
	var data string
	var expiresAt int64

	err := s.db.QueryRowContext(ctx,
		`SELECT data, expires_at FROM sessions WHERE id = ?`, id,
	).Scan(&data, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session %s: %w", id, err)
	}
	if time.Now().Unix() > expiresAt {
		_ = s.Delete(ctx, id)
		return nil, nil
	}

	var values map[string]any
	if err := json.Unmarshal([]byte(data), &values); err != nil {
		return nil, fmt.Errorf("decode session data: %w", err)
	}
	return &Session{ID: id, Values: values}, nil
}

func (s *SQLStore) Save(ctx context.Context, sess *Session) error {
	b, err := json.Marshal(sess.Values)
	if err != nil {
		return fmt.Errorf("encode session data: %w", err)
	}
	expiresAt := time.Now().Add(s.ttl).Unix()

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, data, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, expires_at = excluded.expires_at`,
		sess.ID, string(b), expiresAt,
	)
	if err != nil {
		return fmt.Errorf("save session %s: %w", sess.ID, err)
	}
	return nil
}

func (s *SQLStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session %s: %w", id, err)
	}
	return nil
}
