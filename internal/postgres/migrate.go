package postgres

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate only applies pending migrations; destructive rollback is not exposed.
func Migrate(ctx context.Context, databaseURL string) error {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return errors.New("migration initialization failed")
	}
	return migrate(ctx, databaseURL, files)
}

func migrate(ctx context.Context, databaseURL string, files fs.FS) error {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 30), lock.WithUnlockTimeout(1, 5))
	if err != nil {
		return errors.New("migration lock initialization failed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithSessionLocker(locker))
	if err != nil {
		return errors.New("migration provider initialization failed")
	}
	if _, err := provider.Up(ctx); err != nil {
		return errors.New("database migration failed")
	}
	return nil
}
