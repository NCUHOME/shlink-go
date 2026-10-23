//go:build integration

package testutil

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/redirectrule"
	"github.com/NCUnix/shlink-go/internal/store"
)

func SeedRedirectRules(t *testing.T, db *store.DB, shortURL ShortURLFixture) {
	t.Helper()
	rules := []redirectrule.Rule{
		{
			LongURL:  "https://example.com/english-and-foo-query",
			Priority: 1,
			Conditions: []redirectrule.Condition{
				{Type: "language", MatchValue: strPtr("en")},
				{Type: "any-value-query-param", MatchKey: strPtr("foo")},
			},
		},
		{
			LongURL:  "https://example.com/multiple-query-params",
			Priority: 2,
			Conditions: []redirectrule.Condition{
				{Type: "query-param", MatchKey: strPtr("hello"), MatchValue: strPtr("world")},
				{Type: "query-param", MatchKey: strPtr("foo"), MatchValue: strPtr("bar")},
			},
		},
		{
			LongURL:    "https://example.com/only-english",
			Priority:   3,
			Conditions: []redirectrule.Condition{{Type: "language", MatchValue: strPtr("en")}},
		},
		{
			LongURL:    "android://foo/bar",
			Priority:   4,
			Conditions: []redirectrule.Condition{{Type: "device", MatchValue: strPtr("android")}},
		},
		{
			LongURL:    "fb://profile/33138223345",
			Priority:   5,
			Conditions: []redirectrule.Condition{{Type: "device", MatchValue: strPtr("ios")}},
		},
		{
			LongURL:    "https://example.com/static-ip-address",
			Priority:   6,
			Conditions: []redirectrule.Condition{{Type: "ip-address", MatchValue: strPtr("1.2.3.4")}},
		},
		{
			LongURL:    "https://example.com/linux",
			Priority:   7,
			Conditions: []redirectrule.Condition{{Type: "device", MatchValue: strPtr("linux")}},
		},
	}
	ctx := context.Background()
	for _, rule := range rules {
		conditions, err := json.Marshal(rule.Conditions)
		require.NoError(t, err)
		_, err = db.SQL.ExecContext(
			ctx,
			`INSERT INTO redirect_rules (short_url_id, priority, long_url, conditions) VALUES ($1, $2, $3, $4)`,
			shortURL.ID, rule.Priority, rule.LongURL, conditions,
		)
		require.NoError(t, err)
	}
	_, err := db.SQL.ExecContext(ctx, "UPDATE short_urls SET has_redirect_rules = TRUE WHERE id = $1", shortURL.ID)
	require.NoError(t, err)
}
