package store

import (
	"context"
	"database/sql"
	"errors"
)

var ErrCategoryInUse = errors.New("categoria possui lançamentos; apague ou mova os lançamentos antes")

func scanCategory(r *sql.Rows) (Category, error) {
	var c Category
	err := r.Scan(&c.ID, &c.Name, &c.Bucket)
	return c, err
}

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, bucket FROM categories
		ORDER BY CASE bucket WHEN 'renda' THEN 0 WHEN 'necessidades' THEN 1 WHEN 'desejos' THEN 2 ELSE 3 END, name`)
	return collect(rows, err, scanCategory)
}

func (s *Store) CreateCategory(ctx context.Context, name, bucket string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO categories (name, bucket) VALUES (?, ?)`, name, bucket)
	return err
}

func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	var inUse bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM transactions WHERE category_id = ?)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrCategoryInUse
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id)
	return err
}
