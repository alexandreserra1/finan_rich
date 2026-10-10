package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const debtCols = `id, name, creditor, balance_cents, monthly_rate_bp, min_payment_cents, due_day`

func scanDebt(r pgx.CollectableRow) (Debt, error) {
	var d Debt
	err := r.Scan(&d.ID, &d.Name, &d.Creditor, &d.BalanceCents, &d.MonthlyRateBP, &d.MinPaymentCents, &d.DueDay)
	return d, err
}

// ListDebts devolve as dívidas da mais cara (maior juros) para a mais barata.
func (s *Store) ListDebts(ctx context.Context) ([]Debt, error) {
	rows, err := s.db.Query(ctx, `SELECT `+debtCols+` FROM debts ORDER BY monthly_rate_bp DESC, balance_cents`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanDebt)
}

func (s *Store) GetDebt(ctx context.Context, id int64) (Debt, error) {
	rows, err := s.db.Query(ctx, `SELECT `+debtCols+` FROM debts WHERE id = $1`, id)
	if err != nil {
		return Debt{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanDebt)
}

func (s *Store) SaveDebt(ctx context.Context, d Debt) error {
	if d.ID == 0 {
		_, err := s.db.Exec(ctx, `INSERT INTO debts (name, creditor, balance_cents, monthly_rate_bp, min_payment_cents, due_day)
			VALUES ($1, $2, $3, $4, $5, $6)`, d.Name, d.Creditor, d.BalanceCents, d.MonthlyRateBP, d.MinPaymentCents, d.DueDay)
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE debts SET name = $2, creditor = $3, balance_cents = $4,
		monthly_rate_bp = $5, min_payment_cents = $6, due_day = $7 WHERE id = $1`,
		d.ID, d.Name, d.Creditor, d.BalanceCents, d.MonthlyRateBP, d.MinPaymentCents, d.DueDay)
	return err
}

func (s *Store) DeleteDebt(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM debts WHERE id = $1`, id)
	return err
}
