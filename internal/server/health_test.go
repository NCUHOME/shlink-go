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

func TestHealthWithoutDatabaseReturnsServiceUnavailable(t *testing.T) {
	app := New(config.Config{DefaultDomain: "s.test", CORSAllowedOrigins: []string{"*"}, CORSMaxAge: 3600}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	req := httptest.NewRequest(http.MethodGet, "/rest/health", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"fail"`)
}
