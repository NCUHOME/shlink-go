package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/config"
)

func testServer() *App {
	return New(
		config.Config{DefaultDomain: "s.test", CORSAllowedOrigins: []string{"*"}, CORSMaxAge: 3600},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
	)
}

func TestCORSHeadersAreOnlyPresentWithOrigin(t *testing.T) {
	app := testServer()
	req := httptest.NewRequest(http.MethodGet, "/rest/v3/short-urls", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))

	req = httptest.NewRequest(http.MethodGet, "/rest/v3/short-urls", nil)
	req.Header.Set("Origin", "https://foo.com")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
}

func TestCORSPreflight(t *testing.T) {
	app := testServer()
	req := httptest.NewRequest(http.MethodOptions, "/rest/v3/tags", nil)
	req.Header.Set("Origin", "https://foo.com")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET,DELETE,PUT", rec.Header().Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
	require.NotEmpty(t, rec.Header().Get("Access-Control-Max-Age"))
}
