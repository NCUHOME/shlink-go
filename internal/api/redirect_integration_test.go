//go:build integration

package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/testutil"
)

const (
	androidUserAgent = "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.5414.86 Mobile Safari/537.36"
	iosUserAgent     = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/109.0 Mobile/15E148 Safari/605.1.15"
	windowsUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36 Edg/138.0.3351.95"
	linuxUserAgent   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/81.0.4044.113 Safari/537.36"
)

func TestRedirectRulesMatchUpstreamCases(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	shortURLs := testutil.SeedShortURLs(t, db, fixtures)
	testutil.SeedRedirectRules(t, db, shortURLs["1/def456"])
	app := testutil.NewApp(t, db)

	for _, tc := range []struct {
		name        string
		headers     map[string]string
		query       string
		expectedURL string
	}{
		{"android", map[string]string{"User-Agent": androidUserAgent}, "", "android://foo/bar"},
		{"ios", map[string]string{"User-Agent": iosUserAgent}, "", "fb://profile/33138223345"},
		{"linux", map[string]string{"User-Agent": linuxUserAgent}, "", "https://example.com/linux"},
		{"desktop", map[string]string{"User-Agent": windowsUserAgent}, "", "https://blog.alejandrocelaya.com/2017/12/09/acmailer-7-0-the-most-important-release-in-a-long-time/"},
		{"unknown", nil, "", "https://blog.alejandrocelaya.com/2017/12/09/acmailer-7-0-the-most-important-release-in-a-long-time/"},
		{"english and foo", map[string]string{"Accept-Language": "en-UK"}, "?foo=bar", "https://example.com/english-and-foo-query?foo=bar"},
		{"multiple query params", nil, "?foo=bar&hello=world", "https://example.com/multiple-query-params?foo=bar&hello=world"},
		{"british english", map[string]string{"Accept-Language": "en-UK"}, "", "https://example.com/only-english"},
		{"english", map[string]string{"Accept-Language": "en"}, "", "https://example.com/only-english"},
		{"complex language", map[string]string{"Accept-Language": "fr-FR, es;q=0.9, en;q=0.9, *;q=0.2"}, "", "https://example.com/only-english"},
		{"low quality language", map[string]string{"Accept-Language": "fr-FR, es;q=0.8, en;q=0.5, *;q=0.2"}, "", "https://blog.alejandrocelaya.com/2017/12/09/acmailer-7-0-the-most-important-release-in-a-long-time/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), http.MethodGet, "/def456"+tc.query, requestOptions{Headers: tc.headers})
			require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
			require.Equal(t, tc.expectedURL, rec.Header().Get("Location"))
		})
	}

	for _, header := range []string{
		"CF-Connecting-IP", "X-Forwarded-For", "X-Forwarded",
		"Forwarded", "True-Client-IP", "X-Real-IP",
		"X-Cluster-Client-Ip", "Client-Ip",
	} {
		t.Run("ip header "+header, func(t *testing.T) {
			value := "1.2.3.4"
			if header == "Forwarded" {
				value = "for=1.2.3.4"
			}
			rec := performRequest(t, app.Handler(), http.MethodGet, "/def456", requestOptions{
				Headers: map[string]string{header: value},
			})
			require.Equal(t, http.StatusFound, rec.Code)
			require.Equal(t, "https://example.com/static-ip-address", rec.Header().Get("Location"))
		})
	}
}

func TestRedirectNonHTTPLongURLs(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for index, longURL := range []string{"android://foo/bar", "fb://profile/33138223345", "viber://pa?chatURI=1234"} {
		slug := "non-http-schema-" + string(rune('a'+index))
		rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
			APIKey: "valid_api_key",
			Body:   map[string]any{"longUrl": longURL, "customSlug": slug},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		rec = performRequest(t, app.Handler(), http.MethodGet, "/"+slug, requestOptions{})
		require.Equal(t, http.StatusFound, rec.Code)
		require.Equal(t, longURL, rec.Header().Get("Location"))
	}
}

func TestRedirectQueryForwarding(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"longUrl": "https://example.com", "customSlug": "forward-query-params", "forwardQuery": true},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	rec = performRequest(t, app.Handler(), http.MethodGet, "/forward-query-params?foo%20bar=123", requestOptions{})
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://example.com?foo%20bar=123", rec.Header().Get("Location"))

	rec = performRequest(t, app.Handler(), http.MethodGet, "/custom?foo=bar", requestOptions{})
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://shlink.io", rec.Header().Get("Location"))
}

func TestRedirectExtraPathModes(t *testing.T) {
	for _, mode := range []string{"default", "append", "ignore"} {
		t.Run(mode, func(t *testing.T) {
			db := testutil.NewPostgres(t)
			fixtures := testutil.SeedBase(t, db)
			testutil.SeedShortURLs(t, db, fixtures)
			cfg := testutil.TestConfig()
			cfg.ExtraPathMode = mode
			app := testutil.NewAppWithConfig(t, db, cfg, testutil.TestClock())

			rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
				APIKey: "valid_api_key",
				Body:   map[string]any{"longUrl": "https://example.com/base", "customSlug": "path-mode", "forwardQuery": true},
			})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			rec = performRequest(t, app.Handler(), http.MethodGet, "/path-mode/foo/bar?x=1", requestOptions{})
			switch mode {
			case "default":
				require.Equal(t, http.StatusNotFound, rec.Code)
			case "append":
				require.Equal(t, http.StatusFound, rec.Code)
				require.Equal(t, "https://example.com/base/foo/bar?x=1", rec.Header().Get("Location"))
			case "ignore":
				require.Equal(t, http.StatusFound, rec.Code)
				require.Equal(t, "https://example.com/base?x=1", rec.Header().Get("Location"))
			}
		})
	}
}

func TestRedirectTrailingSlash(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			db := testutil.NewPostgres(t)
			fixtures := testutil.SeedBase(t, db)
			testutil.SeedShortURLs(t, db, fixtures)
			cfg := testutil.TestConfig()
			cfg.ShortURLTrailingSlash = enabled
			app := testutil.NewAppWithConfig(t, db, cfg, testutil.TestClock())

			rec := performRequest(t, app.Handler(), http.MethodGet, "/custom/", requestOptions{})
			if enabled {
				require.Equal(t, http.StatusFound, rec.Code)
			} else {
				require.Equal(t, http.StatusNotFound, rec.Code)
			}
		})
	}
}

func TestRedirectValidSinceAndUntil(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"longUrl": "https://example.com", "customSlug": "future", "validSince": "2021-07-01T00:00:00+00:00"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	rec = performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"longUrl": "https://example.com", "customSlug": "past", "validUntil": "2021-05-01T00:00:00+00:00"},
	})
	require.Equal(t, http.StatusOK, rec.Code)

	for _, slug := range []string{"future", "past"} {
		rec = performRequest(t, app.Handler(), http.MethodGet, "/"+slug, requestOptions{})
		require.Equal(t, http.StatusNotFound, rec.Code, slug)
	}
}

func TestRedirectMaxVisitsAndTracking(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"longUrl": "https://example.com", "customSlug": "limited", "maxVisits": 2},
	})
	require.Equal(t, http.StatusOK, rec.Code)

	// custom has a maxVisits fixture of 2; use a fresh slug to make this test independent.
	for i := 0; i < 2; i++ {
		rec = performRequest(t, app.Handler(), http.MethodGet, "/limited", requestOptions{})
		require.Equal(t, http.StatusFound, rec.Code, "visit %d", i+1)
	}
	rec = performRequest(t, app.Handler(), http.MethodGet, "/limited", requestOptions{})
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/limited/visits", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeBody[visitsPayload](t, rec)
	require.Equal(t, 2, payload.Visits.Pagination.TotalItems)
}

func TestRedirectTrackingDisabledByParam(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	cfg := testutil.TestConfig()
	cfg.DisableTrackParam = "utm_source"
	app := testutil.NewAppWithConfig(t, db, cfg, testutil.TestClock())

	rec := performRequest(t, app.Handler(), http.MethodPost, "/rest/v3/short-urls", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"longUrl": "https://example.com", "customSlug": "no-track"},
	})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/no-track?utm_source=newsletter&foo=bar", requestOptions{})
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://example.com?foo=bar", rec.Header().Get("Location"))

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/short-urls/no-track/visits", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0, decodeBody[visitsPayload](t, rec).Visits.Pagination.TotalItems)
}

func TestRobotsAndTrackRoute(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodGet, "/robots.txt", requestOptions{})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `# For more information about the robots.txt standard, see:
# https://www.robotstxt.org/orig.html

User-agent: *
Allow: /abc123
Allow: /custom
Disallow: /
`, rec.Body.String())

	rec = performRequest(t, app.Handler(), http.MethodGet, "/abc123/track", requestOptions{})
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrphanVisitsAreTracked(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	base := performRequest(t, app.Handler(), http.MethodGet, "/", requestOptions{Host: "s.test"})
	invalid := performRequest(t, app.Handler(), http.MethodGet, "/does-not-exist", requestOptions{Host: "s.test"})
	regular := performRequest(t, app.Handler(), http.MethodGet, "/abc123/extra", requestOptions{Host: "s.test"})
	require.Equal(t, http.StatusNotFound, base.Code)
	require.Equal(t, http.StatusNotFound, invalid.Code)
	require.Equal(t, http.StatusNotFound, regular.Code)

	var count int
	require.NoError(t, db.SQL.QueryRowContext(context.Background(),
		"SELECT count(*) FROM visits WHERE orphan_type IS NOT NULL").Scan(&count))
	require.Equal(t, 3, count)

	for _, orphanType := range []string{"base_url", "invalid_short_url", "regular_404"} {
		rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/visits/orphan?type="+orphanType, requestOptions{APIKey: "valid_api_key"})
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 1, decodeBody[orphanVisitsPayload](t, rec).Visits.Pagination.TotalItems)
	}
}

func TestDomainRedirectsAreAppliedToOrphans(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	rec := performRequest(t, app.Handler(), http.MethodPatch, "/rest/v3/domains/redirects", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"domain": "s.test", "regular404Redirect": "https://fallback.example.com/404", "baseUrlRedirect": "https://fallback.example.com/"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = performRequest(t, app.Handler(), http.MethodGet, "/abc123/extra", requestOptions{Host: "s.test"})
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://fallback.example.com/404", rec.Header().Get("Location"))

	rec = performRequest(t, app.Handler(), http.MethodGet, "/", requestOptions{Host: "s.test"})
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://fallback.example.com/", rec.Header().Get("Location"))
}
