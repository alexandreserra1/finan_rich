package store

import (
	"context"
	"database/sql"
)

const debtCols = `id, name, creditor, balance_cents, monthly_rate_bp, min_payment_cents, due_day`

func scanDebt(r *sql.Rows) (Debt, error) {
	var d Debt
	var due sql.NullInt64
	err := r.Scan(&d.ID, &d.Name, &d.Creditor, &d.BalanceCents, &d.MonthlyRateBP, &d.MinPaymentCents, &due)
	if due.Valid {
		day := int(due.Int64)
		d.DueDay = &day
	}
	return d, err
}

// ListDebts devolve as dívidas da mais cara (maior juros) para a mais barata.
func (s *Store) ListDebts(ctx context.Context) ([]Debt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+debtCols+` FROM debts ORDER BY monthly_rate_bp DESC, balance_cents`)
	return collect(rows, err, scanDebt)
}

func (s *Store) GetDebt(ctx context.Context, id int64) (Debt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+debtCols+` FROM debts WHERE id = ?`, id)
	list, err := collect(rows, err, scanDebt)
	if err != nil {
		return Debt{}, err
	}
	if len(list) == 0 {
		return Debt{}, sql.ErrNoRows
	}
	return list[0], nil
}

func (s *Store) SaveDebt(ctx context.Context, d Debt) error {
	if d.ID == 0 {
		_, err := s.db.ExecContext(ctx, `INSERT INTO debts (name, creditor, balance_cents, monthly_rate_bp, min_payment_cents, due_day)
			VALUES (?, ?, ?, ?, ?, ?)`, d.Name, d.Creditor, d.BalanceCents, d.MonthlyRateBP, d.MinPaymentCents, d.DueDay)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE debts SET name = ?, creditor = ?, balance_cents = ?,
		monthly_rate_bp = ?, min_payment_cents = ?, due_day = ? WHERE id = ?`,
		d.Name, d.Creditor, d.BalanceCents, d.MonthlyRateBP, d.MinPaymentCents, d.DueDay, d.ID)
	return err
}

func (s *Store) DeleteDebt(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM debts WHERE id = ?`, id)
	return err
}
