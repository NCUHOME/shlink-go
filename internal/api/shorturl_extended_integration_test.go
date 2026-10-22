//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/page"
	"github.com/hakimyu/shlink-go/internal/problem"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

type shortURLPayload struct {
	ShortCode     string   `json:"shortCode"`
	ShortURL      string   `json:"shortUrl"`
	LongURL       string   `json:"longUrl"`
	DateCreated   string   `json:"dateCreated"`
	Domain        *string  `json:"domain"`
	Title         *string  `json:"title"`
	Crawlable     bool     `json:"crawlable"`
	ForwardQuery  bool     `json:"forwardQuery"`
	Tags          []string `json:"tags"`
	VisitsSummary struct {
		Total   int64 `json:"total"`
		NonBots int64 `json:"nonBots"`
		Bots    int64 `json:"bots"`
	} `json:"visitsSummary"`
	Meta struct {
		ValidSince *string `json:"validSince"`
		ValidUntil *string `json:"validUntil"`
		MaxVisits  *int    `json:"maxVisits"`
	} `json:"meta"`
	HasRedirectRules bool `json:"hasRedirectRules"`
}

type shortURLListPayload struct {
	ShortURLs struct {
		Data       []shortURLPayload `json:"data"`
		Pagination page.Pagination   `json:"pagination"`
	} `json:"shortUrls"`
}

func shortURLKey(item shortURLPayload) string {
	if item.Domain == nil {
		return "/" + item.ShortCode
	}
	return *item.Domain + "/" + item.ShortCode
}

func TestListShortURLsMatchesUpstreamCases(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	defaultOrder := []string{
		"example.com/ghi789",
		"/custom",
		"/def456",
		"some-domain.com/custom-with-domain",
		"/abc123",
		"/ghi789",
	}

	for _, tc := range []struct {
		name     string
		query    string
		apiKey   string
		expected []string
		total    int
	}{
		{"default order", "", "valid_api_key", defaultOrder, 6},
		{"exclude past valid until", "excludePastValidUntil=true", "valid_api_key", defaultOrder[:5], 5},
		{"exclude max visits reached", "excludeMaxVisitsReached=true", "valid_api_key", []string{"example.com/ghi789", "/custom", "/def456", "some-domain.com/custom-with-domain", "/ghi789"}, 5},
		{"short code asc", "orderBy=shortCode", "valid_api_key", []string{"/abc123", "/custom", "some-domain.com/custom-with-domain", "/def456", "/ghi789", "example.com/ghi789"}, 6},
		{"short code desc", "orderBy=shortCode-DESC", "valid_api_key", []string{"/ghi789", "example.com/ghi789", "/def456", "some-domain.com/custom-with-domain", "/custom", "/abc123"}, 6},
		{"title desc", "orderBy=title-DESC", "valid_api_key", []string{"/abc123", "/def456", "/custom", "/ghi789", "example.com/ghi789", "some-domain.com/custom-with-domain"}, 6},
		{"start date", "startDate=2018-12-01T00:00:00%2B00:00", "valid_api_key", []string{"example.com/ghi789", "/custom", "/def456"}, 3},
		{"end date", "endDate=2018-12-01T00:00:00%2B00:00", "valid_api_key", []string{"some-domain.com/custom-with-domain", "/abc123", "/ghi789"}, 3},
		{"tags foo", "tags[]=foo", "valid_api_key", []string{"example.com/ghi789", "/def456", "/abc123"}, 3},
		{"tags foo bar any", "tags[]=foo&tags[]=bar", "valid_api_key", []string{"example.com/ghi789", "/def456", "/abc123"}, 3},
		{"tags foo bar all", "tags[]=foo&tags[]=bar&tagsMode=all", "valid_api_key", []string{"/def456"}, 1},
		{"tags foo bar baz all", "tags[]=foo&tags[]=bar&tags[]=baz&tagsMode=all", "valid_api_key", []string{}, 0},
		{"search alejandro", "searchTerm=alejandro", "valid_api_key", []string{"example.com/ghi789", "/def456"}, 2},
		{"search cool", "searchTerm=cool", "valid_api_key", []string{"/abc123"}, 1},
		{"search example.com", "searchTerm=example.com", "valid_api_key", []string{"example.com/ghi789"}, 1},
		{"domain example.com", "domain=example.com", "valid_api_key", []string{"example.com/ghi789"}, 1},
		{"domain DEFAULT", "domain=DEFAULT", "valid_api_key", []string{"/custom", "/def456", "/abc123", "/ghi789"}, 4},
		{"exclude tags foo", "excludeTags[]=foo", "valid_api_key", []string{"/custom", "some-domain.com/custom-with-domain", "/ghi789"}, 3},
		{"exclude tags foo bar all", "excludeTags[]=bar&excludeTags[]=foo&excludeTagsMode=all", "valid_api_key", []string{"example.com/ghi789", "/custom", "some-domain.com/custom-with-domain", "/abc123", "/ghi789"}, 5},
		{"filter by api key name", "apiKeyName=author_api_key", "valid_api_key", []string{"/custom", "/def456", "/abc123"}, 3},
		{"author key ignores api key name", "apiKeyName=valid_api_key", "author_api_key", []string{"/custom", "/def456", "/abc123"}, 3},
		{"author key scope", "", "author_api_key", []string{"/custom", "/def456", "/abc123"}, 3},
		{"domain key scope", "", "domain_api_key", []string{"example.com/ghi789"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/rest/v3/short-urls"
			if tc.query != "" {
				path += "?" + tc.query
			}
			rec := performRequest(t, app.Handler(), http.MethodGet, path, requestOptions{APIKey: tc.apiKey})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			payload := decodeBody[shortURLListPayload](t, rec)
			keys := make([]string, 0, len(payload.ShortURLs.Data))
			for _, item := range payload.ShortURLs.Data {
				keys = append(keys, shortURLKey(item))
			}
			require.Equal(t, tc.expected, keys)
			require.Equal(t, tc.total, payload.ShortURLs.Pagination.TotalItems)
			require.Equal(t, 1, payload.ShortURLs.Pagination.CurrentPage)
			require.Equal(t, 10, payload.ShortURLs.Pagination.ItemsPerPage)
			require.Equal(t, len(tc.expected), payload.ShortURLs.Pagination.ItemsInCurrentPage)
		})
	}
}

func TestListShortURLsRejectsInvalidFilters(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls?orderBy=invalid&tagsMode=invalid", requestOptions{APIKey: "valid_api_key"})
	payload := requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
	require.Equal(t, []string{"orderBy", "tagsMode"}, payload.InvalidElements)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls?page=0", requestOptions{APIKey: "valid_api_key"})
	payload = requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
	require.Equal(t, []string{"page"}, payload.InvalidElements)
}

func TestCreateShortURLMatchesUpstreamCases(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	create := func(t *testing.T, body map[string]any, apiKey string) shortURLPayload {
		t.Helper()
		if _, ok := body["longUrl"]; !ok {
			body["longUrl"] = "https://app.shlink.io"
		}
		rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{APIKey: apiKey, Body: body})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeBody[shortURLPayload](t, rec)
	}

	t.Run("only long url", func(t *testing.T) {
		rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
			APIKey: "valid_api_key",
			Body:   map[string]any{"longUrl": "https://app.shlink.io"},
		})
		require.Equal(t, http.StatusOK, rec.Code)
		var raw map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
		for _, key := range []string{"shortCode", "shortUrl", "longUrl", "dateCreated", "tags"} {
			require.Contains(t, raw, key)
		}
	})

	for _, longURL := range []string{"mailto:foo@example.com", "file:///foo/bar", "https://example.com", "shlink://some/path"} {
		t.Run("long url scheme "+longURL, func(t *testing.T) {
			payload := create(t, map[string]any{"longUrl": longURL}, "valid_api_key")
			require.Equal(t, longURL, payload.LongURL)
		})
	}

	t.Run("custom slug and tags", func(t *testing.T) {
		payload := create(t, map[string]any{
			"customSlug": "my cool slug",
			"tags":       []string{"fo o", "  bar", "b az"},
		}, "valid_api_key")
		require.Equal(t, "my-cool-slug", payload.ShortCode)
		require.Equal(t, []string{"fo-o", "bar", "b-az"}, payload.Tags)
	})

	t.Run("emoji", func(t *testing.T) {
		payload := create(t, map[string]any{
			"longUrl":    "https://emojipedia.org/fire/",
			"title":      "🔥🔥🔥",
			"customSlug": "🦣🦣🦣",
		}, "valid_api_key")
		require.Equal(t, "🔥🔥🔥", *payload.Title)
		require.Equal(t, "🦣🦣🦣", payload.ShortCode)
		require.Equal(t, "http://s.test/🦣🦣🦣", payload.ShortURL)
	})

	t.Run("title crop", func(t *testing.T) {
		for _, tc := range []struct {
			title string
			want  string
		}{
			{"foo", "foo"},
			{strings.Repeat(" ", 100) + "bar" + strings.Repeat(" ", 100), "bar"},
			{strings.Repeat("a", 511), strings.Repeat("a", 511)},
			{strings.Repeat("b", 512), strings.Repeat("b", 512)},
			{strings.Repeat("c", 513), strings.Repeat("c", 512)},
		} {
			payload := create(t, map[string]any{"title": tc.title}, "valid_api_key")
			require.Equal(t, tc.want, *payload.Title)
		}
	})

	t.Run("path prefix", func(t *testing.T) {
		for _, slug := range []any{nil, "my-custom-slug"} {
			body := map[string]any{"pathPrefix": "foo/b  ar-baz"}
			if slug != nil {
				body["customSlug"] = slug
			}
			payload := create(t, body, "valid_api_key")
			require.True(t, strings.HasPrefix(payload.ShortCode, "foo-b--ar-baz"), payload.ShortCode)
		}
	})

	t.Run("short code length", func(t *testing.T) {
		payload := create(t, map[string]any{"shortCodeLength": 8}, "valid_api_key")
		require.Len(t, payload.ShortCode, 8)
	})

	t.Run("default domain is dropped", func(t *testing.T) {
		payload := create(t, map[string]any{"domain": "s.test"}, "valid_api_key")
		require.Nil(t, payload.Domain)
	})

	t.Run("domain key enforces domain", func(t *testing.T) {
		for _, domain := range []any{nil, "this-will-be-overwritten.com", "example.com"} {
			body := map[string]any{}
			if domain != nil {
				body["domain"] = domain
			}
			payload := create(t, body, "domain_api_key")
			require.NotNil(t, payload.Domain)
			require.Equal(t, "example.com", *payload.Domain)
		}
	})

	t.Run("find if exists", func(t *testing.T) {
		for _, body := range []map[string]any{
			{"longUrl": "https://www.alejandrocelaya.com"},
			{"longUrl": "https://www.alejandrocelaya.com", "tags": []string{"boo", "far"}},
			{"longUrl": "https://www.alejandrocelaya.com", "customSlug": "find-existing-slug"},
			{"longUrl": "https://www.alejandrocelaya.com", "tags": []string{"boo", "far"}, "validSince": "2020-01-01T00:00:00+00:00", "maxVisits": 7},
		} {
			first := create(t, cloneMap(body), "valid_api_key")
			body["findIfExists"] = true
			second := create(t, cloneMap(body), "valid_api_key")
			require.Equal(t, first.ShortCode, second.ShortCode)
		}

		first := create(t, map[string]any{"longUrl": "https://www.alejandrocelaya.com"}, "valid_api_key")
		second := create(t, map[string]any{"longUrl": "https://www.alejandrocelaya.com/projects", "findIfExists": true}, "valid_api_key")
		require.NotEqual(t, first.ShortCode, second.ShortCode)
	})

	t.Run("invalid input", func(t *testing.T) {
		for _, body := range []map[string]any{
			{},
			{"longUrl": nil},
			{"longUrl": "  "},
			{"longUrl": "foo.com"},
		} {
			rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{APIKey: "valid_api_key", Body: body})
			payload := requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
			require.Contains(t, payload.InvalidElements, "longUrl")
		}
	})

	t.Run("invalid domains", func(t *testing.T) {
		for _, domain := range []string{"localhost:80000", "127.0.0.1", "???/&%$&", "540.38.386.0"} {
			rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
				APIKey: "valid_api_key",
				Body:   map[string]any{"longUrl": "https://example.com", "domain": domain},
			})
			payload := requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
			require.Contains(t, payload.InvalidElements, "domain")
		}
	})

	t.Run("metadata", func(t *testing.T) {
		payload := create(t, map[string]any{
			"validSince": "2020-01-02T00:00:00+00:00",
			"validUntil": "2022-01-02T00:00:00+00:00",
			"maxVisits":  10,
		}, "valid_api_key")
		require.Equal(t, "2020-01-02T00:00:00+00:00", *payload.Meta.ValidSince)
		require.Equal(t, "2022-01-02T00:00:00+00:00", *payload.Meta.ValidUntil)
		require.Equal(t, 10, *payload.Meta.MaxVisits)
	})
}

func TestCreateShortURLDuplicateSlug(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for _, tc := range []struct {
		slug   string
		domain any
	}{
		{"custom", nil},
		{"custom-with-domain", "some-domain.com"},
	} {
		body := map[string]any{"longUrl": "https://example.com/other", "customSlug": tc.slug}
		if tc.domain != nil {
			body["domain"] = tc.domain
		}
		rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{APIKey: "valid_api_key", Body: body})
		payload := requireProblem(t, rec, http.StatusBadRequest, problem.TypeNonUniqueSlug)
		require.Equal(t, tc.slug, payload.CustomSlug)
		if tc.domain == nil {
			require.Empty(t, payload.Domain)
			require.Equal(t, `Provided slug "custom" is already in use.`, payload.Detail)
		} else {
			require.Equal(t, tc.domain, payload.Domain)
			require.Contains(t, payload.Detail, `for domain "some-domain.com"`)
		}
	}

	rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"longUrl": "https://www.alejandrocelaya.com", "customSlug": "custom", "findIfExists": true},
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEditShortURL(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	t.Run("metadata can be reset", func(t *testing.T) {
		rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/short-urls/abc123", requestOptions{
			APIKey: "valid_api_key",
			Body: map[string]any{
				"validSince": "2020-01-01T00:00:00+00:00",
				"validUntil": "2022-01-01T00:00:00+00:00",
				"maxVisits":  20,
			},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		payload := decodeBody[shortURLPayload](t, rec)
		require.NotNil(t, payload.Meta.ValidSince)
		require.NotNil(t, payload.Meta.ValidUntil)
		require.Equal(t, 20, *payload.Meta.MaxVisits)

		rec = performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/short-urls/abc123", requestOptions{
			APIKey: "valid_api_key",
			Body: map[string]any{
				"validSince": nil,
				"validUntil": nil,
				"maxVisits":  nil,
			},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		payload = decodeBody[shortURLPayload](t, rec)
		require.Nil(t, payload.Meta.ValidSince)
		require.Nil(t, payload.Meta.ValidUntil)
		require.Nil(t, payload.Meta.MaxVisits)
	})

	t.Run("long url can be edited", func(t *testing.T) {
		rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/short-urls/abc123", requestOptions{
			APIKey: "valid_api_key",
			Body:   map[string]any{"longUrl": "https://shlink.io/updated"},
		})
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "https://shlink.io/updated", decodeBody[shortURLPayload](t, rec).LongURL)
	})

	t.Run("invalid urls return not found", func(t *testing.T) {
		for _, tc := range []struct {
			path   string
			domain string
		}{
			{"/rest/v3/short-urls/invalid", ""},
			{"/rest/v3/short-urls/abc123?domain=example.com", "example.com"},
			{"/rest/v3/short-urls/custom-with-domain?domain=example.com", "example.com"},
			{"/rest/v3/short-urls/ghi789", ""},
		} {
			apiKey := "valid_api_key"
			if tc.path == "/rest/v3/short-urls/ghi789" {
				apiKey = "author_api_key"
			}
			rec := performRequest(t, app.Handler(), http.MethodPatch, tc.path, requestOptions{APIKey: apiKey, Body: map[string]any{}})
			payload := requireProblem(t, rec, http.StatusNotFound, problem.TypeShortURLNotFound)
			if tc.domain != "" {
				require.Equal(t, tc.domain, payload.Domain)
			}
		}
	})

	t.Run("invalid data", func(t *testing.T) {
		rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/short-urls/invalid", requestOptions{
			APIKey: "valid_api_key",
			Body:   map[string]any{"maxVisits": "not_a_number"},
		})
		requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
	})

	t.Run("domain selects the proper short url", func(t *testing.T) {
		for _, tc := range []struct {
			query       string
			expectedURL string
			domain      *string
		}{
			{"?domain=example.com", "https://blog.alejandrocelaya.com/2019/04/27/considerations-to-properly-use-open-source-software-projects/", stringPtr("example.com")},
			{"", "https://shlink.io/documentation/", nil},
		} {
			rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/short-urls/ghi789"+tc.query, requestOptions{
				APIKey: "valid_api_key",
				Body:   map[string]any{"maxVisits": 100},
			})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			payload := decodeBody[shortURLPayload](t, rec)
			require.Equal(t, tc.expectedURL, payload.LongURL)
			require.Equal(t, 100, *payload.Meta.MaxVisits)
			if tc.domain == nil {
				require.Nil(t, payload.Domain)
			} else {
				require.Equal(t, *tc.domain, *payload.Domain)
			}
		}
	})
}

func TestDeleteShortURL(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	t.Run("invalid urls return not found", func(t *testing.T) {
		for _, tc := range []struct {
			path   string
			domain string
			apiKey string
		}{
			{"/rest/v3/short-urls/invalid", "", "valid_api_key"},
			{"/rest/v3/short-urls/abc123?domain=example.com", "example.com", "valid_api_key"},
			{"/rest/v3/short-urls/custom-with-domain?domain=example.com", "example.com", "valid_api_key"},
			{"/rest/v3/short-urls/ghi789", "", "author_api_key"},
			{"/rest/v3/short-urls/custom-with-domain?domain=some-domain.com", "some-domain.com", "domain_api_key"},
		} {
			rec := performRequest(t, app.Handler(), http.MethodDelete, tc.path, requestOptions{APIKey: tc.apiKey})
			payload := requireProblem(t, rec, http.StatusNotFound, problem.TypeShortURLNotFound)
			if tc.domain != "" {
				require.Equal(t, tc.domain, payload.Domain)
			}
		}
	})

	t.Run("proper short url is deleted when domain is provided", func(t *testing.T) {
		beforeWithDomain := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789?domain=example.com", requestOptions{APIKey: "valid_api_key"})
		beforeWithoutDomain := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789", requestOptions{APIKey: "valid_api_key"})
		require.Equal(t, http.StatusOK, beforeWithDomain.Code)
		require.Equal(t, http.StatusOK, beforeWithoutDomain.Code)

		rec := performRequest(t, app.Handler(), http.MethodDelete, "/rest/v3/short-urls/ghi789?domain=example.com", requestOptions{APIKey: "valid_api_key"})
		require.Equal(t, http.StatusNoContent, rec.Code)
		afterWithDomain := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789?domain=example.com", requestOptions{APIKey: "valid_api_key"})
		afterWithoutDomain := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789", requestOptions{APIKey: "valid_api_key"})
		require.Equal(t, http.StatusNotFound, afterWithDomain.Code)
		require.Equal(t, http.StatusOK, afterWithoutDomain.Code)
	})
}

func TestGetShortURLScopesAndDomainSelection(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789?domain=example.com", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeBody[shortURLPayload](t, rec)
	require.Equal(t, "example.com", *payload.Domain)
	require.Equal(t, []string{"foo"}, payload.Tags)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code)
	payload = decodeBody[shortURLPayload](t, rec)
	require.Nil(t, payload.Domain)
	require.Equal(t, "https://shlink.io/documentation/", payload.LongURL)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/abc123", requestOptions{APIKey: "author_api_key"})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/ghi789", requestOptions{APIKey: "author_api_key"})
	requireProblem(t, rec, http.StatusNotFound, problem.TypeShortURLNotFound)
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func stringPtr(value string) *string {
	return &value
}

func TestFindIfExistsRespectsAPIKeyScope(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	create := func(t *testing.T, body map[string]any, apiKey string) *httptest.ResponseRecorder {
		t.Helper()
		return performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{APIKey: apiKey, Body: body})
	}

	adminRec := create(t, map[string]any{"longUrl": "https://example.com/private", "customSlug": "private-slug"}, "valid_api_key")
	require.Equal(t, http.StatusOK, adminRec.Code, adminRec.Body.String())
	adminCreated := decodeBody[shortURLPayload](t, adminRec)

	// The author-scoped key must not discover the admin's short URL through
	// findIfExists, and must not be able to reuse its custom slug.
	authorRec := create(t, map[string]any{"longUrl": "https://example.com/private", "findIfExists": true}, "author_api_key")
	require.Equal(t, http.StatusOK, authorRec.Code, authorRec.Body.String())
	authorCreated := decodeBody[shortURLPayload](t, authorRec)
	require.NotEqual(t, adminCreated.ShortCode, authorCreated.ShortCode)

	conflictRec := create(t, map[string]any{"longUrl": "https://example.com/private", "customSlug": "private-slug", "findIfExists": true}, "author_api_key")
	requireProblem(t, conflictRec, http.StatusBadRequest, problem.TypeNonUniqueSlug)

	direct := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/"+adminCreated.ShortCode, requestOptions{APIKey: "author_api_key"})
	requireProblem(t, direct, http.StatusNotFound, problem.TypeShortURLNotFound)
}
