package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MustConnect(ctx context.Context, dsn string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}
	return pool
}

// ApplyMigrations: супер-простой мигратор "в одну таблицу".
// Для продакшена часто берут goose/atlas, но здесь без доп.зависимостей.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	if err != nil {
		return err
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		name := filepath.Base(f)

		var exists bool
		if e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, name).Scan(&exists); e != nil {
			return e
		}
		if exists {
			continue
		}

		sqlBytes, e := os.ReadFile(f)
		if e != nil {
			return e
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			return errors.New("empty migration: " + name)
		}

		tx, e := pool.Begin(ctx)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, sqlText)
		if e != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", name, e)
		}
		_, e = tx.Exec(ctx, `INSERT INTO schema_migrations(filename) VALUES($1)`, name)
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if e := tx.Commit(ctx); e != nil {
			return e
		}
	}
	return nil
}
