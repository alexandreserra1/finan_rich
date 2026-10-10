package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListTransactions(ctx context.Context, m Month) ([]Transaction, error) {
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.kind, t.description, t.amount_cents, t.category_id, c.name, c.bucket, t.occurred_on
		FROM transactions t JOIN categories c ON c.id = t.category_id
		WHERE t.occurred_on >= $1 AND t.occurred_on < $2
		ORDER BY t.occurred_on DESC, t.id DESC`, m.Time, m.End())
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Transaction, error) {
		var t Transaction
		err := r.Scan(&t.ID, &t.Kind, &t.Description, &t.AmountCents, &t.CategoryID, &t.CategoryName, &t.Bucket, &t.OccurredOn)
		return t, err
	})
}

func (s *Store) CreateTransaction(ctx context.Context, t Transaction) error {
	_, err := s.db.Exec(ctx, `INSERT INTO transactions (kind, description, amount_cents, category_id, occurred_on)
		VALUES ($1, $2, $3, $4, $5)`, t.Kind, t.Description, t.AmountCents, t.CategoryID, t.OccurredOn)
	return err
}

func (s *Store) DeleteTransaction(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM transactions WHERE id = $1`, id)
	return err
}

// CopyFromPreviousMonth copia os lançamentos do mês anterior para o mês informado
// (útil para salário e contas fixas). Pula os que já existem com a mesma descrição
// e categoria, então pode ser chamado mais de uma vez sem duplicar.
func (s *Store) CopyFromPreviousMonth(ctx context.Context, m Month) (int64, error) {
	prev := m.Prev()
	tag, err := s.db.Exec(ctx, `
		INSERT INTO transactions (kind, description, amount_cents, category_id, occurred_on)
		SELECT p.kind, p.description, p.amount_cents, p.category_id,
		       LEAST(p.occurred_on + INTERVAL '1 month', ($2::date - INTERVAL '1 day'))::date
		FROM transactions p
		WHERE p.occurred_on >= $3 AND p.occurred_on < $1
		  AND NOT EXISTS (
		      SELECT 1 FROM transactions c
		      WHERE c.occurred_on >= $1 AND c.occurred_on < $2
		        AND c.description = p.description AND c.category_id = p.category_id)`,
		m.Time, m.End(), prev.Time)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Totals são os totais de um mês.
type Totals struct {
	Month       Month
	InCents     int64
	OutCents    int64
	BalanceCent int64
}

// MonthlyTotals devolve entradas/saídas de cada mês do ano.
func (s *Store) MonthlyTotals(ctx context.Context, year int) ([]Totals, error) {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	rows, err := s.db.Query(ctx, `
		SELECT date_trunc('month', occurred_on)::date AS mes,
		       COALESCE(SUM(amount_cents) FILTER (WHERE kind = 'entrada'), 0),
		       COALESCE(SUM(amount_cents) FILTER (WHERE kind = 'saida'), 0)
		FROM transactions
		WHERE occurred_on >= $1 AND occurred_on < $2
		GROUP BY mes`, start, start.AddDate(1, 0, 0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byMonth := map[time.Month]Totals{}
	for rows.Next() {
		var mes time.Time
		var t Totals
		if err := rows.Scan(&mes, &t.InCents, &t.OutCents); err != nil {
			return nil, err
		}
		byMonth[mes.Month()] = t
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Totals, 12)
	for i := range out {
		t := byMonth[time.Month(i+1)]
		t.Month = NewMonth(year, time.Month(i+1))
		t.BalanceCent = t.InCents - t.OutCents
		out[i] = t
	}
	return out, nil
}
