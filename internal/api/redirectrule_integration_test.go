//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/problem"
	"github.com/hakimyu/shlink-go/internal/redirectrule"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

func condition(typ string, matchKey, matchValue *string) redirectrule.Condition {
	return redirectrule.Condition{Type: typ, MatchKey: matchKey, MatchValue: matchValue}
}

func TestListRedirectRules(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	shortURLs := testutil.SeedShortURLs(t, db, fixtures)
	testutil.SeedRedirectRules(t, db, shortURLs["1/def456"])
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/invalid/redirect-rules", requestOptions{APIKey: "valid_api_key"})
	payload := requireProblem(t, rec, http.StatusNotFound, problem.TypeShortURLNotFound)
	require.Equal(t, "invalid", payload.ShortCode)
	require.Equal(t, `No URL found with short code "invalid"`, payload.Detail)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/abc123/redirect-rules", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	set := decodeBody[redirectrule.Set](t, rec)
	require.Equal(t, "https://shlink.io", set.DefaultLongURL)
	require.Empty(t, set.RedirectRules)

	expected := []redirectrule.Rule{
		{
			LongURL:  "https://example.com/english-and-foo-query",
			Priority: 1,
			Conditions: []redirectrule.Condition{
				condition("language", nil, stringPtr("en")),
				condition("any-value-query-param", stringPtr("foo"), nil),
			},
		},
		{
			LongURL:  "https://example.com/multiple-query-params",
			Priority: 2,
			Conditions: []redirectrule.Condition{
				condition("query-param", stringPtr("hello"), stringPtr("world")),
				condition("query-param", stringPtr("foo"), stringPtr("bar")),
			},
		},
		{
			LongURL:    "https://example.com/only-english",
			Priority:   3,
			Conditions: []redirectrule.Condition{condition("language", nil, stringPtr("en"))},
		},
		{
			LongURL:    "android://foo/bar",
			Priority:   4,
			Conditions: []redirectrule.Condition{condition("device", nil, stringPtr("android"))},
		},
		{
			LongURL:    "fb://profile/33138223345",
			Priority:   5,
			Conditions: []redirectrule.Condition{condition("device", nil, stringPtr("ios"))},
		},
		{
			LongURL:    "https://example.com/static-ip-address",
			Priority:   6,
			Conditions: []redirectrule.Condition{condition("ip-address", nil, stringPtr("1.2.3.4"))},
		},
		{
			LongURL:    "https://example.com/linux",
			Priority:   7,
			Conditions: []redirectrule.Condition{condition("device", nil, stringPtr("linux"))},
		},
	}
	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/def456/redirect-rules", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	set = decodeBody[redirectrule.Set](t, rec)
	require.Equal(t, "https://blog.alejandrocelaya.com/2017/12/09/acmailer-7-0-the-most-important-release-in-a-long-time/", set.DefaultLongURL)
	require.Equal(t, expected, set.RedirectRules)
}

func TestSetRedirectRules(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	shortURLs := testutil.SeedShortURLs(t, db, fixtures)
	testutil.SeedRedirectRules(t, db, shortURLs["1/def456"])
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls/invalid/redirect-rules", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"redirectRules": []any{}},
	})
	payload := requireProblem(t, rec, http.StatusNotFound, problem.TypeShortURLNotFound)
	require.Equal(t, "invalid", payload.ShortCode)

	for _, body := range []any{
		map[string]any{"redirectRules": []string{"foo"}},
		map[string]any{"redirectRules": []map[string]any{{"longUrl": "invalid"}}},
		map[string]any{"redirectRules": []map[string]any{{"longUrl": "https://example.com", "conditions": "foo"}}},
		map[string]any{"redirectRules": []map[string]any{{
			"longUrl": "https://example.com",
			"conditions": []map[string]any{{
				"type": "invalid", "matchKey": nil, "matchValue": "foo",
			}},
		}}},
		map[string]any{"redirectRules": []map[string]any{{
			"longUrl": "https://example.com",
			"conditions": []map[string]any{{
				"type": "device", "matchKey": nil, "matchValue": "invalid-device",
			}},
		}}},
		map[string]any{"redirectRules": []map[string]any{{
			"longUrl": "https://example.com",
			"conditions": []map[string]any{{
				"type": "ip-address", "matchKey": nil, "matchValue": "not an IP address",
			}},
		}}},
		map[string]any{"redirectRules": []map[string]any{{
			"longUrl": "https://example.com",
			"conditions": []map[string]any{{
				"type": "geolocation-country-code", "matchKey": nil, "matchValue": "not a country code",
			}},
		}}},
	} {
		rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls/abc123/redirect-rules", requestOptions{
			APIKey: "valid_api_key",
			Body:   body,
		})
		requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
	}

	rules := []map[string]any{
		{
			"longUrl": "https://example.com/english-and-foo-query",
			"conditions": []map[string]any{
				{"type": "language", "matchKey": nil, "matchValue": "en"},
				{"type": "any-value-query-param", "matchKey": "foo", "matchValue": nil},
			},
		},
		{
			"longUrl": "https://example.com/multiple-query-params",
			"conditions": []map[string]any{
				{"type": "query-param", "matchKey": "hello", "matchValue": "world"},
				{"type": "query-param", "matchKey": "foo", "matchValue": "bar"},
			},
		},
	}
	rec = performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls/abc123/redirect-rules", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"redirectRules": rules},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	set := decodeBody[redirectrule.Set](t, rec)
	require.Equal(t, "https://shlink.io", set.DefaultLongURL)
	require.Len(t, set.RedirectRules, 2)
	require.Equal(t, 1, set.RedirectRules[0].Priority)
	require.Equal(t, 2, set.RedirectRules[1].Priority)

	// Empty list removes all rules and clears the flag.
	rec = performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls/abc123/redirect-rules", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"redirectRules": []any{}},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, decodeBody[redirectrule.Set](t, rec).RedirectRules)

	for _, pattern := range []string{"1.2.3.4", "1.2.3.0/24", "1.2.3.*", "1.2.*.4"} {
		rec = performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls/abc123/redirect-rules", requestOptions{
			APIKey: "valid_api_key",
			Body: map[string]any{"redirectRules": []map[string]any{{
				"longUrl": "https://example.com",
				"conditions": []map[string]any{{
					"type": "ip-address", "matchKey": nil, "matchValue": pattern,
				}},
			}}},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	rec = performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls/abc123/redirect-rules", requestOptions{
		APIKey: "valid_api_key",
		Body: map[string]any{"redirectRules": []map[string]any{{
			"longUrl":    "https://example.com",
			"conditions": []map[string]any{{"type": "geolocation-country-code", "matchKey": nil, "matchValue": "US"}},
		}}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
