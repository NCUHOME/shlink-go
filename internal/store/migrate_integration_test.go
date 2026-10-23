//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/migrate"

	"github.com/NCUnix/shlink-go/internal/store"
	"github.com/NCUnix/shlink-go/internal/testutil"
	"github.com/NCUnix/shlink-go/migrations"
)

func TestMigrationsCanRollbackAndRunAgain(t *testing.T) {
	db := testutil.NewPostgres(t)
	ctx := context.Background()

	m := migrate.NewMigrations()
	require.NoError(t, m.Discover(migrations.FS))
	migrator := migrate.NewMigrator(db.DB, m)

	_, err := migrator.Rollback(ctx)
	require.NoError(t, err)
	var exists bool
	require.NoError(t, db.SQL.QueryRowContext(ctx,
		"SELECT to_regclass('public.short_urls') IS NOT NULL").Scan(&exists))
	require.False(t, exists)

	_, err = migrator.Migrate(ctx)
	require.NoError(t, err)
	require.NoError(t, db.SQL.QueryRowContext(ctx,
		"SELECT to_regclass('public.short_urls') IS NOT NULL").Scan(&exists))
	require.True(t, exists)

	require.NoError(t, store.Migrate(ctx, db))
}
