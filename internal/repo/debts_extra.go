package repo

import (
	"context"
	"time"
)

type SummaryRow struct {
	Currency     string
	YouLentCents int64
	YouOweCents  int64
	NetCents     int64
}

// Если у тебя Debts уже объявлен в debts.go — удали этот struct и оставь только методы ниже.
// Важно: методы должны иметь receiver *(Debts)

func (r *Debts) ListDebtors(ctx context.Context, ownerID int64, limit int) ([]DebtRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
				SELECT
			d.id,
			d.amount_cents,
			d.currency,
			d.due_date,
			u.username,
			u.first_name,
			u.last_name
		FROM debts d
		JOIN users u
		  ON u.id = d.debtor_id
		JOIN contacts c
		  ON c.owner_user_id = d.creditor_id
		 AND c.contact_user_id = d.debtor_id
		WHERE d.creditor_id = $1
		  AND d.status = 'active'
		ORDER BY d.due_date
		LIMIT $2;
	`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DebtRow, 0, 32)
	for rows.Next() {
		var d DebtRow
		if err := rows.Scan(
			&d.ID,
			&d.AmountCents,
			&d.Currency,
			&d.DueDate,
			&d.Name,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Debts) ListMyDebts(ctx context.Context, ownerID int64, limit int) ([]DebtRow, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			d.id,
			d.amount_cents,
			d.currency,
			d.due_date,
			COALESCE(u.first_name || ' ' || u.last_name, '@' || u.username)
		FROM debts d
		JOIN users u ON u.id = d.debtor_id
		WHERE d.creditor_id = $1
		  AND d.status = 'active'
		ORDER BY d.due_date
		LIMIT $2;
	`, ownerID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DebtRow, 0, 32)
	for rows.Next() {
		var (
			id        int64
			amount    int64
			currency  string
			dueDate   time.Time
			username  *string
			firstName *string
			lastName  *string
		)

		if err := rows.Scan(
			&id,
			&amount,
			&currency,
			&dueDate,
			&username,
			&firstName,
			&lastName,
		); err != nil {
			return nil, err
		}

		name := ""
		if firstName != nil {
			name = *firstName
		}
		if lastName != nil {
			name += " " + *lastName
		}
		if name == "" && username != nil {
			name = "@" + *username
		}

		out = append(out, DebtRow{
			ID:          id,
			AmountCents: amount,
			Currency:    currency,
			DueDate:     dueDate,
			Name:        name,
		})
	}

	return out, rows.Err()
}

func (r *Debts) SummaryByCurrency(ctx context.Context, ownerID int64) ([]SummaryRow, error) {
	rows, err := r.pool.Query(ctx, `
		WITH lent AS (
			SELECT d.currency, COALESCE(SUM(d.amount_cents),0) AS cents
			FROM debts d
			WHERE d.creditor_id = $1
			  AND d.status = 'active'
			GROUP BY d.currency
		),
		owe AS (
			SELECT d.currency, COALESCE(SUM(d.amount_cents),0) AS cents
			FROM debts d
			WHERE d.debtor_id = $1
			  AND d.status = 'active'
			GROUP BY d.currency
		),
		allc AS (
			SELECT currency FROM lent
			UNION
			SELECT currency FROM owe
		)
		SELECT a.currency,
		       COALESCE(l.cents,0) AS you_lent,
		       COALESCE(o.cents,0) AS you_owe,
		       COALESCE(l.cents,0) - COALESCE(o.cents,0) AS net
		FROM allc a
		LEFT JOIN lent l ON l.currency = a.currency
		LEFT JOIN owe  o ON o.currency = a.currency
		ORDER BY a.currency
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SummaryRow, 0, 8)
	for rows.Next() {
		var s SummaryRow
		if err := rows.Scan(
			&s.Currency,
			&s.YouLentCents,
			&s.YouOweCents,
			&s.NetCents,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Закрытие долга: разрешим закрывать кредитору или должнику (любая сторона)
func (r *Debts) CloseDebt(ctx context.Context, ownerID, debtID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE debts
		SET status = 'closed', closed_at = NOW()
		WHERE id = $1
		AND d.status = 'active'
		  AND (creditor_id = $2 OR debtor_id = $2)
	`, debtID, ownerID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
