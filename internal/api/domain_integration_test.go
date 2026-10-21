//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/problem"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

type redirectsPayload struct {
	BaseURLRedirect         *string `json:"baseUrlRedirect"`
	Regular404Redirect      *string `json:"regular404Redirect"`
	InvalidShortURLRedirect *string `json:"invalidShortUrlRedirect"`
}

type domainPayload struct {
	Domain    string           `json:"domain"`
	IsDefault bool             `json:"isDefault"`
	Redirects redirectsPayload `json:"redirects"`
}

type domainsPayload struct {
	Domains struct {
		Data             []domainPayload  `json:"data"`
		DefaultRedirects redirectsPayload `json:"defaultRedirects"`
	} `json:"domains"`
}

func TestListDomainsMatchesUpstreamCases(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	emptyRedirects := redirectsPayload{}
	detachedRedirects := redirectsPayload{
		BaseURLRedirect:    stringPtr("foo.com"),
		Regular404Redirect: stringPtr("bar.com"),
	}

	for _, tc := range []struct {
		name   string
		apiKey string
		data   []domainPayload
	}{
		{
			name: "admin", apiKey: "valid_api_key",
			data: []domainPayload{
				{Domain: "s.test", IsDefault: true, Redirects: emptyRedirects},
				{Domain: "detached-with-redirects.com", Redirects: detachedRedirects},
				{Domain: "example.com", Redirects: emptyRedirects},
				{Domain: "some-domain.com", Redirects: emptyRedirects},
			},
		},
		{
			name: "author", apiKey: "author_api_key",
			data: []domainPayload{
				{Domain: "s.test", IsDefault: true, Redirects: emptyRedirects},
			},
		},
		{
			name: "domain", apiKey: "domain_api_key",
			data: []domainPayload{
				{Domain: "example.com", Redirects: emptyRedirects},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/domains", requestOptions{APIKey: tc.apiKey})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			payload := decodeBody[domainsPayload](t, rec)
			require.Equal(t, tc.data, payload.Domains.Data)
			require.Equal(t, emptyRedirects, payload.Domains.DefaultRedirects)
		})
	}
}

func TestDomainRedirects(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for _, body := range []any{
		map[string]any{},
		map[string]any{"domain": ""},
		map[string]any{"domain": nil},
		map[string]any{"domain": "not a domain"},
		map[string]any{"domain": "540.38.386.0"},
		map[string]any{"domain": "example.com:888888"},
		[]any{},
	} {
		rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/domains/redirects", requestOptions{
			APIKey: "valid_api_key",
			Body:   body,
		})
		requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
	}

	for _, tc := range []struct {
		name     string
		body     map[string]any
		expected redirectsPayload
	}{
		{
			name: "new domain",
			body: map[string]any{"domain": "my-new-domain.com", "regular404Redirect": "foo.com"},
			expected: redirectsPayload{
				Regular404Redirect: stringPtr("foo.com"),
			},
		},
		{
			name: "default domain",
			body: map[string]any{"domain": "s.test", "regular404Redirect": "foo-for-default.com"},
			expected: redirectsPayload{
				Regular404Redirect: stringPtr("foo-for-default.com"),
			},
		},
		{
			name: "existing domain with redirects",
			body: map[string]any{
				"domain":                  "detached-with-redirects.com",
				"baseUrlRedirect":         nil,
				"invalidShortUrlRedirect": "foo.com",
			},
			expected: redirectsPayload{
				Regular404Redirect:      stringPtr("bar.com"),
				InvalidShortURLRedirect: stringPtr("foo.com"),
			},
		},
		{
			name: "existing domain with no redirects",
			body: map[string]any{
				"domain":                  "example.com",
				"baseUrlRedirect":         nil,
				"invalidShortUrlRedirect": "foo.com",
			},
			expected: redirectsPayload{
				InvalidShortURLRedirect: stringPtr("foo.com"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/domains/redirects", requestOptions{
				APIKey: "valid_api_key",
				Body:   tc.body,
			})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tc.expected, decodeBody[redirectsPayload](t, rec))
		})
	}
}
