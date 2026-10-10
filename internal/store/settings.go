package store

import (
	"context"
	"database/sql"
	"strconv"
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

const splitPrefix = "pct_"

// Split devolve o percentual planejado da renda para cada bucket de gasto.
func (s *Store) Split(ctx context.Context) (map[string]int, error) {
	raw, err := s.Settings(ctx, splitPrefix)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(raw))
	for k, v := range raw {
		out[k], _ = strconv.Atoi(v)
	}
	return out, nil
}

func (s *Store) SaveSplit(ctx context.Context, split map[string]int) error {
	raw := make(map[string]string, len(split))
	for k, v := range split {
		raw[k] = strconv.Itoa(v)
	}
	return s.SaveSettings(ctx, splitPrefix, raw)
}
