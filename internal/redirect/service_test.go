package redirect

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/shorturl"
)

func TestBuildRedirectURLForwardsQuery(t *testing.T) {
	request := httptest.NewRequest("GET", "http://s.test/abc?foo=bar&utm_source=x", nil)
	got := buildRedirectURL("https://example.com/path?existing=1", request, true, []string{"utm_source"})
	require.Equal(t, "https://example.com/path?existing=1&foo=bar", got)

	got = buildRedirectURL("https://example.com/path", request, false, nil)
	require.Equal(t, "https://example.com/path", got)
}

func TestAppendPath(t *testing.T) {
	require.Equal(t, "https://example.com/base/foo/bar", appendPath("https://example.com/base", "/foo/bar"))
	require.Equal(t, "https://example.com/base/foo", appendPath("https://example.com/base/", "/foo"))
}

func TestResolvePlaceholders(t *testing.T) {
	request := httptest.NewRequest("GET", "http://s.test/foo/bar", nil)
	got := resolvePlaceholders(request, "https://example.com/{DOMAIN}{ORIGINAL_PATH}")
	require.Equal(t, "https://example.com/s.test/foo/bar", got)
}

func TestIsActive(t *testing.T) {
	now := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	maxVisits := 2

	require.True(t, isActive(&shorturl.Resolved{}, now))
	require.False(t, isActive(&shorturl.Resolved{ValidSince: &future}, now))
	require.False(t, isActive(&shorturl.Resolved{ValidUntil: &past}, now))
	require.False(t, isActive(&shorturl.Resolved{MaxVisits: &maxVisits, VisitsTotal: 2}, now))
	require.True(t, isActive(&shorturl.Resolved{MaxVisits: &maxVisits, VisitsTotal: 1}, now))
}

func TestRequestAuthority(t *testing.T) {
	request := httptest.NewRequest("GET", "http://example.com:8080/foo", nil)
	require.Equal(t, "example.com", requestAuthority(request))
}
