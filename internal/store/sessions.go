package store

import (
	"context"
	"time"
)

const timeLayout = time.RFC3339

func (s *Store) CreateSession(ctx context.Context, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, expires_at) VALUES (?, ?)`,
		tokenHash, expires.UTC().Format(timeLayout))
	return err
}

// SessionValid diz se a sessão existe e não expirou.
func (s *Store) SessionValid(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sessions WHERE token_hash = ? AND expires_at > ?)`,
		tokenHash, now.UTC().Format(timeLayout)).Scan(&ok)
	return ok, err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteAllSessions desconecta todos os aparelhos (usado ao trocar a senha).
func (s *Store) DeleteAllSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions`)
	return err
}

// DeleteExpiredSessions remove sessões vencidas.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.UTC().Format(timeLayout))
	return err
}
