package store

import (
	"context"
	"database/sql"
	"time"
)

// maxAccessLog é quantos eventos o registro de acesso guarda (o mais antigo sai).
const maxAccessLog = 200

// AccessEvent é uma linha do registro de acesso.
type AccessEvent struct {
	At    time.Time
	Event string // código do evento (internal/web: accessEvents)
	IP    string
}

// LogAccess grava um evento de segurança e apaga os que passam do teto.
func (s *Store) LogAccess(ctx context.Context, event, ip string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO access_log (event, ip) VALUES (?, ?)`, event, ip); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM access_log WHERE id <= (SELECT MAX(id) FROM access_log) - ?`, maxAccessLog)
		return err
	})
}

// AccessLog devolve os eventos mais recentes primeiro.
func (s *Store) AccessLog(ctx context.Context, limit int) ([]AccessEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT at, event, ip FROM access_log ORDER BY id DESC LIMIT ?`, limit)
	return collect(rows, err, func(r *sql.Rows) (AccessEvent, error) {
		var e AccessEvent
		var at string
		err := r.Scan(&at, &e.Event, &e.IP)
		if err == nil {
			e.At, err = time.Parse(time.RFC3339, at)
		}
		return e, err
	})
}
