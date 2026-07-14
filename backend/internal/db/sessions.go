package db

import (
	"database/sql"
	"errors"
)

// Session is a server-side login session keyed by an opaque token.
type Session struct {
	Token     string
	UserID    int64
	ExpiresAt int64
}

// CreateSession stores a new session token.
func (db *DB) CreateSession(token string, userID, createdAt, expiresAt int64) error {
	_, err := db.Exec(
		`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		token, userID, createdAt, expiresAt,
	)
	return err
}

// GetSession returns a non-expired session, or ErrNotFound.
func (db *DB) GetSession(token string, now int64) (*Session, error) {
	var s Session
	err := db.QueryRow(
		`SELECT token, user_id, expires_at FROM sessions WHERE token = ? AND expires_at > ?`,
		token, now,
	).Scan(&s.Token, &s.UserID, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// TouchSession extends a session's expiry (sliding window).
func (db *DB) TouchSession(token string, expiresAt int64) error {
	_, err := db.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, expiresAt, token)
	return err
}

// DeleteSession removes a single session (logout).
func (db *DB) DeleteSession(token string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions purges sessions past their expiry.
func (db *DB) DeleteExpiredSessions(now int64) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now)
	return err
}
