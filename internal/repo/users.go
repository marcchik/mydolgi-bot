package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Users struct{ pool *pgxpool.Pool }

func NewUsers(p *pgxpool.Pool) *Users { return &Users{pool: p} }

func (r *Users) UpsertTelegramUser(ctx context.Context, telegramID int64, username, firstName, lastName *string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users(telegram_id, username, first_name, last_name)
		VALUES($1,$2,$3,$4)
		ON CONFLICT (telegram_id) DO UPDATE
		SET username=EXCLUDED.username,
			first_name=EXCLUDED.first_name,
			last_name=EXCLUDED.last_name
		RETURNING id
	`, telegramID, username, firstName, lastName).Scan(&id)
	return id, err
}

func (r *Users) GetByTelegramID(ctx context.Context, telegramID int64) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM users WHERE telegram_id=$1`, telegramID).Scan(&id)
	return id, err
}

func (r *Users) GetTelegramIDByUserID(ctx context.Context, userID int64) (int64, error) {
	var tid int64
	err := r.pool.QueryRow(ctx, `SELECT telegram_id FROM users WHERE id=$1`, userID).Scan(&tid)
	return tid, err
}

func (r *Users) FindByUsername(ctx context.Context, username string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(
		ctx,
		`SELECT id FROM users WHERE lower(username) = lower($1)`,
		username,
	).Scan(&id)
	return id, err
}

func (r *Users) GetUserIDByTelegramID(ctx context.Context, telegramID int64) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE telegram_id = $1`,
		telegramID,
	).Scan(&id)
	return id, err
}
