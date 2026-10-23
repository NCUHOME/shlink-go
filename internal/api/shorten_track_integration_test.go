//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/problem"
	"github.com/NCUnix/shlink-go/internal/shorturl"
	"github.com/NCUnix/shlink-go/internal/testutil"
)

func TestShortenURLSupportsQueryAuthenticationAndFormats(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	longURL := "https://example.com/shorten"
	query := url.Values{
		"apiKey":  {"valid_api_key"},
		"longUrl": {longURL},
		"format":  {"json"},
	}
	rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/shorten?"+query.Encode(), requestOptions{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var created shorturl.ShortURL
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, longURL, created.LongURL)
	require.NotEmpty(t, created.ShortCode)

	query.Set("format", "txt")
	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/shorten?"+query.Encode(), requestOptions{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	require.Regexp(t, `^http://s\.test/[A-Za-z0-9]+$`, rec.Body.String())

	query.Del("format")
	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/shorten?"+query.Encode(), requestOptions{
		Headers: map[string]string{"Accept": "text/plain"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	require.NotEmpty(t, rec.Body.String())
}

func TestShortenURLRequiresQueryAPIKey(t *testing.T) {
	db := testutil.NewPostgres(t)
	testutil.SeedBase(t, db)
	app := testutil.NewApp(t, db)

	query := url.Values{"longUrl": {"https://example.com/shorten"}}
	rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/shorten?"+query.Encode(), requestOptions{})
	p := requireProblem(t, rec, http.StatusUnauthorized, problem.TypeMissingAuthentication)
	require.Equal(t, `Expected authentication to be provided in "apiKey" query param`, p.Detail)
}

func TestTrackEndpointReturnsPixelAndRecordsVisit(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodGet, "/custom/track", requestOptions{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "image/gif", rec.Header().Get("Content-Type"))
	require.NotEmpty(t, rec.Body.Bytes())

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/custom/visits", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, decodeBody[visitsPayload](t, rec).Visits.Pagination.TotalItems)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/does-not-exist/track", requestOptions{})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/gif", rec.Header().Get("Content-Type"))
}
