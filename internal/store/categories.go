package store

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCategoryInUse = errors.New("categoria possui lançamentos; apague ou mova os lançamentos antes")

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, bucket FROM categories
		ORDER BY CASE bucket WHEN 'renda' THEN 0 WHEN 'necessidades' THEN 1 WHEN 'desejos' THEN 2 ELSE 3 END, name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Category, error) {
		var c Category
		err := r.Scan(&c.ID, &c.Name, &c.Bucket)
		return c, err
	})
}

func (s *Store) CreateCategory(ctx context.Context, name, bucket string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO categories (name, bucket) VALUES ($1, $2)`, name, bucket)
	return err
}

func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
		return ErrCategoryInUse
	}
	return err
}

// Split devolve o percentual planejado da renda para cada bucket de gasto.
func (s *Store) Split(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.Query(ctx, `SELECT key, value FROM settings WHERE key LIKE 'pct_%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		n, _ := strconv.Atoi(v)
		out[k[len("pct_"):]] = n
	}
	return out, rows.Err()
}

func (s *Store) SaveSplit(ctx context.Context, split map[string]int) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		for bucket, pct := range split {
			if _, err := tx.Exec(ctx, `INSERT INTO settings (key, value) VALUES ($1, $2)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, "pct_"+bucket, strconv.Itoa(pct)); err != nil {
				return err
			}
		}
		return nil
	})
}
