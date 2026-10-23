package store

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/migrate"

	"github.com/NCUnix/shlink-go/migrations"
)

func Migrate(ctx context.Context, db *DB) error {
	m := migrate.NewMigrations()
	if err := m.Discover(migrations.FS); err != nil {
		return fmt.Errorf("discover migrations: %w", err)
	}
	migrator := migrate.NewMigrator(db.DB, m)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	if err := migrator.Lock(ctx); err != nil {
		return fmt.Errorf("lock migrator: %w", err)
	}
	defer func() { _ = migrator.Unlock(ctx) }()
	if _, err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
