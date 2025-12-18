package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Debts struct{ pool *pgxpool.Pool }

func NewDebts(p *pgxpool.Pool) *Debts { return &Debts{pool: p} }

func (r *Debts) CreateDebt(ctx context.Context, creditorID, debtorID int64, amountCents int64, currency string, dueDate time.Time) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO debts(creditor_id, debtor_id, amount_cents, currency, due_date)
		VALUES($1,$2,$3,$4,$5)
		RETURNING id
	`, creditorID, debtorID, amountCents, currency, dueDate.Format("2006-01-02")).Scan(&id)
	return id, err
}

func (r *Debts) MarkOverdue(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE debts
		SET status='overdue', updated_at=now()
		WHERE status='active' AND due_date < CURRENT_DATE
	`)
	return err
}

type DueDebt struct {
	ID          int64
	CreditorID  int64
	DebtorID    int64
	AmountCents int64
	Currency    string
	DueDate     time.Time
	Status      string
}

type DebtRow struct {
	ID          int64
	AmountCents int64
	Currency    string
	DueDate     time.Time
	Name        string // имя контрагента (должник или кредитор)
}

// Возвращает активные долги, у которых due_date находится на (today + offsetDays)
func (r *Debts) GetDebtsDueOnOffset(ctx context.Context, offsetDays int) ([]DueDebt, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, creditor_id, debtor_id, amount_cents, currency, due_date, status
		FROM debts
		WHERE status='active'
		  AND due_date = (CURRENT_DATE + $1::int)
	`, offsetDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DueDebt
	for rows.Next() {
		var d DueDebt
		if e := rows.Scan(&d.ID, &d.CreditorID, &d.DebtorID, &d.AmountCents, &d.Currency, &d.DueDate, &d.Status); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, nil
}
