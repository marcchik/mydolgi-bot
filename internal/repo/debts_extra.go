package repo

import (
	"context"
	"time"
)

type DebtRow struct {
	ID          int64
	CreditorID  int64
	DebtorID    int64
	AmountCents int64
	Currency    string
	DueDate     time.Time
	Status      string
	ClosedAt    *time.Time
	DebtorName  string // resolved for lists
}

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
		SELECT d.id, d.creditor_id, d.debtor_id, d.amount_cents, d.currency, d.due_date, d.status, d.closed_at,
		       COALESCE(NULLIF(TRIM(a.alias), ''), COALESCE(NULLIF(u.username, ''), CONCAT_WS(' ', u.first_name, u.last_name))) AS debtor_name
		FROM debts d
		LEFT JOIN users u ON u.id = d.debtor_id
		LEFT JOIN LATERAL (
			SELECT alias
			FROM contact_aliases
			WHERE owner_id = $1 AND contact_user_id = d.debtor_id
			ORDER BY LENGTH(alias) DESC
			LIMIT 1
		) a ON true
		WHERE d.creditor_id = $1
		  AND d.status = 'active'
		ORDER BY (d.status = 'active') DESC, d.due_date ASC, d.id DESC
		LIMIT $2
	`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DebtRow, 0, 32)
	for rows.Next() {
		var d DebtRow
		if err := rows.Scan(&d.ID, &d.CreditorID, &d.DebtorID, &d.AmountCents, &d.Currency, &d.DueDate, &d.Status, &d.ClosedAt, &d.DebtorName); err != nil {
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
		SELECT d.id, d.creditor_id, d.debtor_id, d.amount_cents, d.currency, d.due_date, d.status, d.closed_at,
		       COALESCE(NULLIF(TRIM(a.alias), ''), COALESCE(NULLIF(u.username, ''), CONCAT_WS(' ', u.first_name, u.last_name))) AS creditor_name
		FROM debts d
		LEFT JOIN users u ON u.id = d.creditor_id
		LEFT JOIN LATERAL (
			SELECT alias
			FROM contact_aliases
			WHERE owner_id = $1 AND contact_user_id = d.creditor_id
			ORDER BY LENGTH(alias) DESC
			LIMIT 1
		) a ON true
		WHERE d.debtor_id = $1
		  AND d.status = 'active'
		ORDER BY (d.status = 'active') DESC, d.due_date ASC, d.id DESC
		LIMIT $2
	`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DebtRow, 0, 32)
	for rows.Next() {
		var d DebtRow
		if err := rows.Scan(&d.ID, &d.CreditorID, &d.DebtorID, &d.AmountCents, &d.Currency, &d.DueDate, &d.Status, &d.ClosedAt, &d.DebtorName); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Debts) SummaryByCurrency(ctx context.Context, ownerID int64) ([]SummaryRow, error) {
	rows, err := r.pool.Query(ctx, `
		WITH lent AS (
			SELECT currency, COALESCE(SUM(amount_cents),0) AS cents
			FROM debts
			WHERE creditor_id = $1 AND d.status = 'active'
			GROUP BY currency
		),
		owe AS (
			SELECT currency, COALESCE(SUM(amount_cents),0) AS cents
			FROM debts
			WHERE debtor_id = $1 AND d.status = 'active'
			GROUP BY currency
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
		if err := rows.Scan(&s.Currency, &s.YouLentCents, &s.YouOweCents, &s.NetCents); err != nil {
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
