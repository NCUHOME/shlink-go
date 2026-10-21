//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/problem"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

func TestShortURLListMatchesUpstreamFixture(t *testing.T) {
	db := testutil.NewPostgres(t)
	f := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, f)
	app := testutil.NewApp(t, db)

	rec := request(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls", "valid_api_key", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		ShortURLs struct {
			Data       []map[string]any `json:"data"`
			Pagination map[string]any   `json:"pagination"`
		} `json:"shortUrls"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.ShortURLs.Data, 6)
	require.Equal(t, "ghi789", payload.ShortURLs.Data[0]["shortCode"])
	require.Equal(t, "example.com", payload.ShortURLs.Data[0]["domain"])
	require.Equal(t, float64(1), payload.ShortURLs.Pagination["currentPage"])
	require.Equal(t, float64(6), payload.ShortURLs.Pagination["totalItems"])
	require.Equal(t, float64(6), payload.ShortURLs.Pagination["itemsInCurrentPage"])
}

func TestShortURLListFiltersTagsAndScope(t *testing.T) {
	db := testutil.NewPostgres(t)
	f := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, f)
	app := testutil.NewApp(t, db)

	rec := request(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls?tags[]=foo", "valid_api_key", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		ShortURLs struct {
			Data []map[string]any `json:"data"`
		} `json:"shortUrls"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.ShortURLs.Data, 3)

	rec = request(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls", "author_api_key", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.ShortURLs.Data, 3)
}

func TestCreateGetEditDeleteShortURL(t *testing.T) {
	db := testutil.NewPostgres(t)
	f := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, f)
	app := testutil.NewApp(t, db)

	rec := request(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", "valid_api_key", map[string]any{
		"longUrl":    "https://example.com",
		"customSlug": "my cool slug",
		"tags":       []string{"Foo", "bar"},
		"maxVisits":  5,
	})
	t.Log(rec.Body.String())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	shortCode, _ := created["shortCode"].(string)
	require.Equal(t, "my-cool-slug", shortCode)
	require.Equal(t, "http://s.test/my-cool-slug", created["shortUrl"])
	require.Equal(t, []any{"foo", "bar"}, created["tags"])

	rec = request(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/"+shortCode, "valid_api_key", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = request(t, app.Handler(), http.MethodPatch, "/rest/v3/short-urls/"+shortCode, "valid_api_key", map[string]any{
		"longUrl":   "https://example.org",
		"maxVisits": nil,
		"tags":      []string{"baz"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var edited map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &edited))
	require.Equal(t, "https://example.org", edited["longUrl"])
	require.Equal(t, []any{"baz"}, edited["tags"])
	require.Nil(t, edited["meta"].(map[string]any)["maxVisits"])

	rec = request(t, app.Handler(), http.MethodDelete, "/rest/v3/short-urls/"+shortCode, "valid_api_key", nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	rec = request(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/"+shortCode, "valid_api_key", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateShortURLRejectsDuplicateSlug(t *testing.T) {
	db := testutil.NewPostgres(t)
	f := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, f)
	app := testutil.NewApp(t, db)

	rec := request(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", "valid_api_key", map[string]any{
		"longUrl":    "https://example.com/other",
		"customSlug": "abc123",
	})
	t.Log(rec.Body.String())
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, problem.TypeNonUniqueSlug, payload["type"])
	require.Equal(t, "abc123", payload["customSlug"])
}

func TestMissingAPIKeyReturnsProblem(t *testing.T) {
	db := testutil.NewPostgres(t)
	testutil.SeedBase(t, db)
	app := testutil.NewApp(t, db)

	rec := request(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, problem.TypeMissingAuthentication, payload["type"])
}

func request(t *testing.T, app http.Handler, method, path, apiKey string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}
