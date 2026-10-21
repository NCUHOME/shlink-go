//go:build integration

package shlinkmigrate_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/apikey"
	"github.com/hakimyu/shlink-go/internal/shlinkmigrate"
	"github.com/hakimyu/shlink-go/internal/store"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

func TestRunImportsShlinkData(t *testing.T) {
	target := testutil.NewPostgres(t)
	source := testutil.NewPostgresRaw(t)
	createSourceSchema(t, source)
	seedSourceData(t, source)

	result, err := shlinkmigrate.Run(t.Context(), target, source, shlinkmigrate.Options{
		DefaultDomain: "s.test",
		BatchSize:     1,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Domains)
	require.Equal(t, int64(2), result.APIKeys)
	require.Equal(t, int64(1), result.APIKeyRoles)
	require.Equal(t, int64(2), result.Tags)
	require.Equal(t, int64(1), result.ShortURLs)
	require.Equal(t, int64(2), result.ShortURLTags)
	require.Equal(t, int64(2), result.Visits)
	require.Equal(t, int64(1), result.RedirectRules)

	var domain struct {
		Authority string `bun:"authority"`
		IsDefault bool   `bun:"is_default"`
	}
	require.NoError(t, target.NewSelect().Table("domains").Where("authority = ?", "s.test").Scan(t.Context(), &domain))
	require.True(t, domain.IsDefault)

	var shortURL struct {
		LongURL          string `bun:"long_url"`
		VisitsTotal      int64  `bun:"visits_total"`
		HasRedirectRules bool   `bun:"has_redirect_rules"`
	}
	require.NoError(t, target.NewSelect().Table("short_urls").Where("short_code = ?", "abc123").Scan(t.Context(), &shortURL))
	require.Equal(t, "https://example.com/imported", shortURL.LongURL)
	require.Equal(t, int64(1), shortURL.VisitsTotal)
	require.True(t, shortURL.HasRedirectRules)

	var shortURLDomainID int64
	require.NoError(t, target.NewRaw("SELECT domain_id FROM short_urls WHERE short_code = ?", "abc123").Scan(t.Context(), &shortURLDomainID))
	require.NotZero(t, shortURLDomainID)

	var visitDomainID int64
	require.NoError(t, target.NewRaw("SELECT domain_id FROM visits WHERE id = ?", 100).Scan(t.Context(), &visitDomainID))
	require.Equal(t, shortURLDomainID, visitDomainID)

	var tagCount int
	require.NoError(t, target.NewRaw(`
		SELECT count(*) FROM short_url_tags st
		JOIN tags t ON t.id = st.tag_id
		JOIN short_urls s ON s.id = st.short_url_id
		WHERE s.short_code = ?`, "abc123").Scan(t.Context(), &tagCount))
	require.Equal(t, 2, tagCount)

	var conditions string
	require.NoError(t, target.NewRaw(`
		SELECT conditions::text FROM redirect_rules r
		JOIN short_urls s ON s.id = r.short_url_id
		WHERE s.short_code = ?`, "abc123").Scan(t.Context(), &conditions))
	var decoded []map[string]any
	require.NoError(t, json.Unmarshal([]byte(conditions), &decoded))
	require.Len(t, decoded, 1)
	require.Equal(t, "device", decoded[0]["type"])
	require.Equal(t, "mobile", decoded[0]["matchValue"])

	var orphanVisits int
	require.NoError(t, target.NewRaw("SELECT count(*) FROM visits WHERE orphan_type = ?", "regular_404").Scan(t.Context(), &orphanVisits))
	require.Equal(t, 1, orphanVisits)

	auth := apikey.NewAuthenticator(target.DB)
	principal, err := auth.Authenticate(t.Context(), "valid_api_key")
	require.NoError(t, err)
	require.Equal(t, "Imported key", principal.Name)

	_, err = shlinkmigrate.Run(t.Context(), target, source, shlinkmigrate.Options{DefaultDomain: "s.test"})
	require.ErrorIs(t, err, shlinkmigrate.ErrTargetNotEmpty)
}

func createSourceSchema(t *testing.T, db *store.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE domains (
			id BIGSERIAL PRIMARY KEY,
			authority TEXT NOT NULL UNIQUE,
			base_url_redirect TEXT,
			regular_not_found_redirect TEXT,
			invalid_short_url_redirect TEXT
		)`,
		`CREATE TABLE api_keys (
			id BIGSERIAL PRIMARY KEY,
			"key" TEXT NOT NULL UNIQUE,
			name TEXT,
			expiration_date TIMESTAMPTZ,
			enabled BOOLEAN NOT NULL DEFAULT TRUE
		)`,
		`CREATE TABLE api_key_roles (
			id BIGSERIAL PRIMARY KEY,
			role_name TEXT NOT NULL,
			meta JSON NOT NULL DEFAULT '{}'::json,
			api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
			UNIQUE (role_name, api_key_id)
		)`,
		`CREATE TABLE tags (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE short_urls (
			id BIGSERIAL PRIMARY KEY,
			domain_id BIGINT REFERENCES domains(id),
			short_code TEXT NOT NULL,
			original_url TEXT NOT NULL,
			title TEXT,
			date_created TIMESTAMPTZ NOT NULL,
			valid_since TIMESTAMPTZ,
			valid_until TIMESTAMPTZ,
			max_visits INTEGER,
			crawlable BOOLEAN NOT NULL DEFAULT FALSE,
			forward_query BOOLEAN NOT NULL DEFAULT TRUE,
			author_api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE short_urls_in_tags (
			short_url_id BIGINT NOT NULL REFERENCES short_urls(id) ON DELETE CASCADE,
			tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (short_url_id, tag_id)
		)`,
		`CREATE TABLE visit_locations (
			id BIGSERIAL PRIMARY KEY,
			country_code TEXT,
			country_name TEXT,
			region_name TEXT,
			city_name TEXT,
			timezone TEXT,
			lat DOUBLE PRECISION NOT NULL,
			lon DOUBLE PRECISION NOT NULL,
			is_empty BOOLEAN NOT NULL DEFAULT FALSE
		)`,
		`CREATE TABLE visits (
			id BIGSERIAL PRIMARY KEY,
			short_url_id BIGINT REFERENCES short_urls(id) ON DELETE CASCADE,
			date TIMESTAMPTZ NOT NULL,
			referer TEXT,
			user_agent TEXT,
			remote_addr TEXT,
			visited_url TEXT,
			redirect_url TEXT,
			potential_bot BOOLEAN NOT NULL DEFAULT FALSE,
			type TEXT NOT NULL,
			visit_location_id BIGINT REFERENCES visit_locations(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE short_url_visits_counts (
			id BIGSERIAL PRIMARY KEY,
			short_url_id BIGINT REFERENCES short_urls(id) ON DELETE CASCADE,
			potential_bot BOOLEAN NOT NULL DEFAULT FALSE,
			"count" BIGINT NOT NULL DEFAULT 1,
			slot_id INTEGER NOT NULL,
			UNIQUE (short_url_id, potential_bot, slot_id)
		)`,
		`CREATE TABLE short_url_redirect_rules (
			id BIGSERIAL PRIMARY KEY,
			short_url_id BIGINT NOT NULL REFERENCES short_urls(id) ON DELETE CASCADE,
			priority INTEGER NOT NULL,
			long_url TEXT NOT NULL
		)`,
		`CREATE TABLE redirect_conditions (
			id BIGSERIAL PRIMARY KEY,
			type TEXT NOT NULL,
			match_key TEXT,
			match_value TEXT
		)`,
		`CREATE TABLE redirect_conditions_in_short_url_redirect_rules (
			short_url_redirect_rule_id BIGINT NOT NULL REFERENCES short_url_redirect_rules(id) ON DELETE CASCADE,
			redirect_condition_id BIGINT NOT NULL REFERENCES redirect_conditions(id) ON DELETE CASCADE,
			PRIMARY KEY (short_url_redirect_rule_id, redirect_condition_id)
		)`,
	}
	for _, statement := range statements {
		_, err := db.SQL.ExecContext(context.Background(), statement)
		require.NoError(t, err, statement)
	}
}

func seedSourceData(t *testing.T, db *store.DB) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO domains (id, authority, regular_not_found_redirect) VALUES (1, 's.test', 'https://example.com/404'), (2, 'example.com', NULL)`, nil},
		{`INSERT INTO api_keys (id, "key", name, enabled) VALUES (1, $1, 'Imported key', TRUE), (2, $2, 'Imported key', TRUE)`, []any{
			apikey.Hash("valid_api_key"),
			apikey.Hash("second_api_key"),
		}},
		{`INSERT INTO api_key_roles (id, role_name, meta, api_key_id) VALUES (1, 'AUTHORED_SHORT_URLS', '{}'::json, 1)`, nil},
		{`INSERT INTO tags (id, name) VALUES (1, 'foo'), (2, 'bar')`, nil},
		{`INSERT INTO short_urls
			(id, domain_id, short_code, original_url, title, date_created, crawlable, forward_query, author_api_key_id)
		 VALUES (10, NULL, 'abc123', 'https://example.com/imported', 'Imported', '2025-01-02T03:04:05Z', TRUE, TRUE, 1)`, nil},
		{`INSERT INTO short_urls_in_tags (short_url_id, tag_id) VALUES (10, 1), (10, 2)`, nil},
		{`INSERT INTO visit_locations
			(id, country_code, country_name, region_name, city_name, timezone, lat, lon, is_empty)
		 VALUES (20, 'CN', 'China', 'Shanghai', 'Shanghai', 'Asia/Shanghai', 31.2304, 121.4737, FALSE)`, nil},
		{`INSERT INTO visits
			(id, short_url_id, date, referer, user_agent, remote_addr, visited_url, potential_bot, type, visit_location_id)
		 VALUES (100, 10, '2025-02-03T04:05:06Z', 'https://ref.example', 'shlink-tests-agent', '1.2.3.4',
		         'https://s.test/abc123', FALSE, 'valid_short_url', 20)`, nil},
		{`INSERT INTO visits
			(id, short_url_id, date, user_agent, remote_addr, visited_url, potential_bot, type)
		 VALUES (101, NULL, '2025-02-04T04:05:06Z', 'shlink-tests-agent', '1.2.3.5',
		         'https://s.test/missing', FALSE, 'regular_404')`, nil},
		{`INSERT INTO short_url_visits_counts (short_url_id, potential_bot, "count", slot_id) VALUES (10, FALSE, 1, 1)`, nil},
		{`INSERT INTO short_url_redirect_rules (id, short_url_id, priority, long_url)
		 VALUES (30, 10, 1, 'https://example.com/mobile')`, nil},
		{`INSERT INTO redirect_conditions (id, type, match_key, match_value)
		 VALUES (40, 'device', NULL, 'mobile')`, nil},
		{`INSERT INTO redirect_conditions_in_short_url_redirect_rules
			(short_url_redirect_rule_id, redirect_condition_id) VALUES (30, 40)`, nil},
	}
	for _, statement := range statements {
		_, err := db.SQL.ExecContext(context.Background(), statement.query, statement.args...)
		require.NoError(t, err, statement.query)
	}
}
