//go:build integration

package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/NCUnix/shlink-go/internal/store"
)

func NewPostgres(t *testing.T) *store.DB {
	t.Helper()
	db := newPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	require.NoError(t, store.Migrate(ctx, db))
	return db
}

func NewPostgresRaw(t *testing.T) *store.DB {
	t.Helper()
	return newPostgres(t)
}

func newPostgres(t *testing.T) *store.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	container, err := postgres.Run(
		ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("shlink_test"),
		postgres.WithUsername("shlink"),
		postgres.WithPassword("shlink"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := store.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
