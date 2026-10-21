//go:build integration

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/store"
)

type ShortURLFixture struct {
	ID               int64
	DomainID         int64
	ShortCode        string
	LongURL          string
	Title            *string
	DateCreated      time.Time
	ValidSince       *time.Time
	ValidUntil       *time.Time
	MaxVisits        *int
	Crawlable        bool
	ForwardQuery     bool
	AuthorAPIKeyID   *int64
	HasRedirectRules bool
	VisitsTotal      int64
	VisitsNonBots    int64
	VisitsBots       int64
	Tags             []string
}

func SeedShortURLs(t *testing.T, db *store.DB, f Fixtures) map[string]ShortURLFixture {
	t.Helper()
	date := func(value string) time.Time {
		parsed, err := time.Parse(time.RFC3339, value)
		require.NoError(t, err)
		return parsed
	}
	ptr := func(value string) *string { return &value }
	intPtr := func(value int) *int { return &value }

	authorID := f.APIKeyIDs["author_api_key"]
	items := []ShortURLFixture{
		{
			DomainID: f.DefaultDomainID, ShortCode: "abc123", LongURL: "https://shlink.io",
			Title: ptr("My cool title"), DateCreated: date("2018-05-01T00:00:00+00:00"),
			MaxVisits: intPtr(2), Crawlable: true, ForwardQuery: true,
			AuthorAPIKeyID: &authorID, VisitsTotal: 3, VisitsNonBots: 3, Tags: []string{"foo"},
		},
		{
			DomainID: f.DefaultDomainID, ShortCode: "def456",
			LongURL:      "https://blog.alejandrocelaya.com/2017/12/09/acmailer-7-0-the-most-important-release-in-a-long-time/",
			DateCreated:  date("2019-01-01T00:00:10+00:00"),
			ValidSince:   func() *time.Time { v := date("2020-05-01T00:00:00+00:00"); return &v }(),
			ForwardQuery: true, AuthorAPIKeyID: &authorID, HasRedirectRules: true,
			VisitsTotal: 2, VisitsNonBots: 1, VisitsBots: 1, Tags: []string{"foo", "bar"},
		},
		{
			DomainID: f.DefaultDomainID, ShortCode: "custom", LongURL: "https://shlink.io",
			DateCreated: date("2019-01-01T00:00:20+00:00"), MaxVisits: intPtr(2),
			Crawlable: true, ForwardQuery: false, AuthorAPIKeyID: &authorID,
		},
		{
			DomainID: f.DefaultDomainID, ShortCode: "ghi789", LongURL: "https://shlink.io/documentation/",
			DateCreated:  date("2018-05-01T00:00:00+00:00"),
			ValidUntil:   func() *time.Time { v := date("2020-05-01T00:00:00+00:00"); return &v }(),
			ForwardQuery: true, VisitsTotal: 2, VisitsNonBots: 2,
		},
		{
			DomainID: f.ExampleDomainID, ShortCode: "ghi789",
			LongURL:     "https://blog.alejandrocelaya.com/2019/04/27/considerations-to-properly-use-open-source-software-projects/",
			DateCreated: date("2019-01-01T00:00:30+00:00"), ForwardQuery: true, Tags: []string{"foo"},
		},
		{
			DomainID: f.SomeDomainID, ShortCode: "custom-with-domain", LongURL: "https://google.com",
			DateCreated: date("2018-10-20T00:00:00+00:00"), ForwardQuery: true,
		},
	}

	result := make(map[string]ShortURLFixture, len(items))
	for _, item := range items {
		item.ID = insertShortURL(t, db, item)
		for _, tag := range item.Tags {
			require.NoError(t, attachTag(t, db, item.ID, f.TagIDs[tag]))
		}
		result[fmt.Sprintf("%d/%s", item.DomainID, item.ShortCode)] = item
	}
	return result
}

func insertShortURL(t *testing.T, db *store.DB, item ShortURLFixture) int64 {
	t.Helper()
	var id int64
	err := db.SQL.QueryRowContext(
		context.Background(),
		`INSERT INTO short_urls
			(domain_id, short_code, long_url, title, date_created, valid_since, valid_until, max_visits,
			 crawlable, forward_query, author_api_key_id, has_redirect_rules,
			 visits_total, visits_non_bots, visits_bots)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 RETURNING id`,
		item.DomainID, item.ShortCode, item.LongURL, item.Title, item.DateCreated,
		item.ValidSince, item.ValidUntil, item.MaxVisits, item.Crawlable, item.ForwardQuery,
		item.AuthorAPIKeyID, item.HasRedirectRules, item.VisitsTotal, item.VisitsNonBots, item.VisitsBots,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func attachTag(t *testing.T, db *store.DB, shortURLID, tagID int64) error {
	t.Helper()
	_, err := db.SQL.ExecContext(
		context.Background(),
		`INSERT INTO short_url_tags (short_url_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		shortURLID, tagID,
	)
	return err
}
