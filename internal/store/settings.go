package store

import (
	"context"
	"database/sql"
	"strings"
)

// Settings devolve todas as configurações cujo nome começa com prefix (sem o prefixo).
func (s *Store) Settings(ctx context.Context, prefix string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings WHERE key LIKE ? || '%'`, prefix)
	type kv struct{ k, v string }
	list, err := collect(rows, err, func(r *sql.Rows) (kv, error) {
		var p kv
		err := r.Scan(&p.k, &p.v)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list))
	for _, p := range list {
		out[strings.TrimPrefix(p.k, prefix)] = p.v
	}
	return out, nil
}

// SaveSettings grava os pares informados, adicionando prefix ao nome de cada um.
func (s *Store) SaveSettings(ctx context.Context, prefix string, values map[string]string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for k, v := range values {
			if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)
				ON CONFLICT (key) DO UPDATE SET value = excluded.value`, prefix+k, v); err != nil {
				return err
			}
		}
		return nil
	})
}
