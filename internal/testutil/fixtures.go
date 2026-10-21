//go:build integration

package testutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/apikey"
	"github.com/hakimyu/shlink-go/internal/store"
)

type Fixtures struct {
	DefaultDomainID         int64
	ExampleDomainID         int64
	SomeDomainID            int64
	DetachedDomainID        int64
	DetachedWithRedirectsID int64
	APIKeyIDs               map[string]int64
	TagIDs                  map[string]int64
}

func Reset(t *testing.T, db *store.DB) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{
		"redirect_rules", "visits", "short_url_tags", "tags", "short_urls",
		"api_key_roles", "api_keys", "domains",
	} {
		_, err := db.SQL.ExecContext(ctx, "TRUNCATE TABLE "+table+" RESTART IDENTITY CASCADE")
		require.NoError(t, err)
	}
}

func SeedBase(t *testing.T, db *store.DB) Fixtures {
	t.Helper()
	Reset(t, db)

	f := Fixtures{APIKeyIDs: map[string]int64{}, TagIDs: map[string]int64{}}
	f.DefaultDomainID = insertDomain(t, db, "s.test", true, nil, nil, nil)
	f.ExampleDomainID = insertDomain(t, db, "example.com", false, nil, nil, nil)
	f.SomeDomainID = insertDomain(t, db, "some-domain.com", false, nil, nil, nil)
	f.DetachedDomainID = insertDomain(t, db, "this_domain_is_detached.com", false, nil, nil, nil)
	f.DetachedWithRedirectsID = insertDomain(t, db, "detached-with-redirects.com", false, strPtr("foo.com"), strPtr("bar.com"), nil)

	f.APIKeyIDs["valid_api_key"] = insertAPIKey(t, db, "valid_api_key", true, nil)
	f.APIKeyIDs["disabled_api_key"] = insertAPIKey(t, db, "disabled_api_key", false, nil)
	expired := time.Now().Add(-24 * time.Hour)
	f.APIKeyIDs["expired_api_key"] = insertAPIKey(t, db, "expired_api_key", true, &expired)
	f.APIKeyIDs["author_api_key"] = insertAPIKey(t, db, "author_api_key", true, nil)
	f.APIKeyIDs["domain_api_key"] = insertAPIKey(t, db, "domain_api_key", true, nil)
	f.APIKeyIDs["no_orphans_api_key"] = insertAPIKey(t, db, "no_orphans_api_key", true, nil)

	insertRole(t, db, f.APIKeyIDs["author_api_key"], "AUTHORED_SHORT_URLS", nil)
	insertRole(t, db, f.APIKeyIDs["domain_api_key"], "DOMAIN_SPECIFIC", map[string]any{
		"domain_id": f.ExampleDomainID,
		"authority": "example.com",
	})
	insertRole(t, db, f.APIKeyIDs["no_orphans_api_key"], "NO_ORPHAN_VISITS", nil)

	for _, name := range []string{"foo", "bar", "baz"} {
		f.TagIDs[name] = insertTag(t, db, name)
	}

	return f
}

func insertDomain(t *testing.T, db *store.DB, authority string, isDefault bool, base, regular404, invalid *string) int64 {
	t.Helper()
	var id int64
	err := db.SQL.QueryRowContext(
		context.Background(),
		`INSERT INTO domains (authority, is_default, base_url_redirect, regular_404_redirect, invalid_short_url_redirect)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		authority, isDefault, base, regular404, invalid,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertAPIKey(t *testing.T, db *store.DB, raw string, enabled bool, expiresAt *time.Time) int64 {
	t.Helper()
	var id int64
	err := db.SQL.QueryRowContext(
		context.Background(),
		`INSERT INTO api_keys (name, key_hash, enabled, expires_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		raw, apikey.Hash(raw), enabled, expiresAt,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertRole(t *testing.T, db *store.DB, apiKeyID int64, role string, meta map[string]any) {
	t.Helper()
	if meta == nil {
		meta = map[string]any{}
	}
	payload, err := json.Marshal(meta)
	require.NoError(t, err)
	_, err = db.SQL.ExecContext(
		context.Background(),
		`INSERT INTO api_key_roles (api_key_id, role, meta) VALUES ($1, $2, $3)`,
		apiKeyID, role, payload,
	)
	require.NoError(t, err)
}

func insertTag(t *testing.T, db *store.DB, name string) int64 {
	t.Helper()
	var id int64
	err := db.SQL.QueryRowContext(
		context.Background(),
		`INSERT INTO tags (name) VALUES ($1) RETURNING id`,
		name,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func strPtr(s string) *string { return &s }
