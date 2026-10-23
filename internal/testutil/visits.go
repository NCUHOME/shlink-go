//go:build integration

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/store"
)

func SeedVisits(t *testing.T, db *store.DB, f Fixtures, shortURLs map[string]ShortURLFixture) {
	t.Helper()
	ctx := context.Background()
	date := func(value string) time.Time {
		parsed, err := time.Parse(time.RFC3339, value)
		require.NoError(t, err)
		return parsed
	}
	abc := shortURLs[shortURLKey(f.DefaultDomainID, "abc123")]
	def := shortURLs[shortURLKey(f.DefaultDomainID, "def456")]
	ghi := shortURLs[shortURLKey(f.DefaultDomainID, "ghi789")]

	insertVisit := func(shortURLID, domainID int64, at time.Time, referer, userAgent, visitedURL string, bot bool) {
		_, err := db.SQL.ExecContext(
			ctx,
			`INSERT INTO visits
				(short_url_id, domain_id, visited_at, referer, user_agent, remote_ip, visited_url, potential_bot)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			shortURLID, domainID, at, referer, userAgent, "1.2.3.4", visitedURL, bot,
		)
		require.NoError(t, err)
	}

	insertVisit(abc.ID, f.DefaultDomainID, date("2021-01-01T00:00:00+00:00"), "", "shlink-tests-agent", "http://s.test/abc123", false)
	insertVisit(abc.ID, f.DefaultDomainID, date("2021-01-02T00:00:00+00:00"), "https://google.com", "shlink-tests-agent", "http://s.test/abc123", false)
	insertVisit(abc.ID, f.DefaultDomainID, date("2021-01-03T00:00:00+00:00"), "", "shlink-tests-agent", "http://s.test/abc123", false)
	insertVisit(def.ID, f.DefaultDomainID, date("2021-02-01T00:00:00+00:00"), "", "cf-facebook", "http://s.test/def456", true)
	insertVisit(def.ID, f.DefaultDomainID, date("2021-02-02T00:00:00+00:00"), "https://app.shlink.io", "shlink-tests-agent", "http://s.test/def456", false)
	insertVisit(ghi.ID, f.DefaultDomainID, date("2021-03-01T00:00:00+00:00"), "", "shlink-tests-agent", "http://s.test/ghi789", false)
	insertVisit(ghi.ID, f.DefaultDomainID, date("2021-03-02T00:00:00+00:00"), "https://app.shlink.io", "shlink-tests-agent", "http://s.test/ghi789", false)

	insertOrphan := func(domainID int64, at time.Time, referer, userAgent, visitedURL, orphanType string, bot bool) {
		_, err := db.SQL.ExecContext(
			ctx,
			`INSERT INTO visits
				(domain_id, visited_at, referer, user_agent, remote_ip, visited_url, potential_bot, orphan_type)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			domainID, at, referer, userAgent, "1.2.3.4", visitedURL, bot, orphanType,
		)
		require.NoError(t, err)
	}
	insertOrphan(f.DefaultDomainID, date("2020-01-01T00:00:00+00:00"), "https://s.test", "shlink-tests-agent", "https://s.test/foo", "base_url", false)
	insertOrphan(f.DefaultDomainID, date("2020-02-01T00:00:00+00:00"), "https://s.test/foo/bar", "shlink-tests-agent", "https://s.test/bar", "regular_404", false)
	insertOrphan(f.ExampleDomainID, date("2020-03-01T00:00:00+00:00"), "https://s.test/foo", "cf-facebook", "https://example.com/short", "invalid_short_url", true)
}

func shortURLKey(domainID int64, shortCode string) string {
	return fmt.Sprintf("%d/%s", domainID, shortCode)
}
