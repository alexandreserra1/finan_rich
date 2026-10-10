package store

import (
	"context"
	"database/sql"
	"time"
)

const dateLayout = "2006-01-02"

func scanTransaction(r *sql.Rows) (Transaction, error) {
	var t Transaction
	var on string
	if err := r.Scan(&t.ID, &t.Kind, &t.Description, &t.AmountCents, &t.CategoryID, &t.CategoryName, &t.Bucket, &on); err != nil {
		return t, err
	}
	var err error
	t.OccurredOn, err = time.Parse(dateLayout, on)
	return t, err
}

func (s *Store) ListTransactions(ctx context.Context, m Month) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.kind, t.description, t.amount_cents, t.category_id, c.name, c.bucket, t.occurred_on
		FROM transactions t JOIN categories c ON c.id = t.category_id
		WHERE t.occurred_on >= ? AND t.occurred_on < ?
		ORDER BY t.occurred_on DESC, t.id DESC`, m.Format(dateLayout), m.End().Format(dateLayout))
	return collect(rows, err, scanTransaction)
}

func (s *Store) CreateTransaction(ctx context.Context, t Transaction) error {
	return insertTransaction(ctx, s.db, t)
}

// execer é satisfeito por *sql.DB e *sql.Tx, para reaproveitar o insert dentro de transações.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertTransaction(ctx context.Context, db execer, t Transaction) error {
	_, err := db.ExecContext(ctx, `INSERT INTO transactions (kind, description, amount_cents, category_id, occurred_on)
		VALUES (?, ?, ?, ?, ?)`, t.Kind, t.Description, t.AmountCents, t.CategoryID, t.OccurredOn.Format(dateLayout))
	return err
}

func (s *Store) DeleteTransaction(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, id)
	return err
}

// CopyFromPreviousMonth copia os lançamentos do mês anterior para o mês informado
// (útil para salário e contas fixas). Pula os que já existem com a mesma descrição
// e categoria, então pode ser chamado mais de uma vez sem duplicar.
func (s *Store) CopyFromPreviousMonth(ctx context.Context, m Month) (int, error) {
	prev, err := s.ListTransactions(ctx, m.Prev())
	if err != nil {
		return 0, err
	}
	current, err := s.ListTransactions(ctx, m)
	if err != nil {
		return 0, err
	}
	type key struct {
		desc string
		cat  int64
	}
	exists := make(map[key]bool, len(current))
	for _, t := range current {
		exists[key{t.Description, t.CategoryID}] = true
	}
	copied := 0
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		for _, t := range prev {
			k := key{t.Description, t.CategoryID}
			if exists[k] {
				continue
			}
			exists[k] = true
			t.OccurredOn = m.Day(t.OccurredOn.Day())
			if err := insertTransaction(ctx, tx, t); err != nil {
				return err
			}
			copied++
		}
		return nil
	})
	return copied, err
}

// Totals são os totais de um mês.
type Totals struct {
	Month        Month
	InCents      int64
	OutCents     int64
	BalanceCents int64
}

// MonthlyTotals devolve entradas/saídas de cada um dos 12 meses do ano.
func (s *Store) MonthlyTotals(ctx context.Context, year int) ([]Totals, error) {
	start := NewMonth(year, time.January)
	rows, err := s.db.QueryContext(ctx, `
		SELECT CAST(strftime('%m', occurred_on) AS INTEGER) AS mes,
		       COALESCE(SUM(CASE WHEN kind = 'entrada' THEN amount_cents END), 0),
		       COALESCE(SUM(CASE WHEN kind = 'saida' THEN amount_cents END), 0)
		FROM transactions
		WHERE occurred_on >= ? AND occurred_on < ?
		GROUP BY mes`, start.Format(dateLayout), start.AddDate(1, 0, 0).Format(dateLayout))
	type row struct {
		month   int
		in, out int64
	}
	list, err := collect(rows, err, func(r *sql.Rows) (row, error) {
		var x row
		err := r.Scan(&x.month, &x.in, &x.out)
		return x, err
	})
	if err != nil {
		return nil, err
	}
	out := make([]Totals, 12)
	for i := range out {
		out[i].Month = NewMonth(year, time.Month(i+1))
	}
	for _, x := range list {
		t := &out[x.month-1]
		t.InCents, t.OutCents, t.BalanceCents = x.in, x.out, x.in-x.out
	}
	return out, nil
}
