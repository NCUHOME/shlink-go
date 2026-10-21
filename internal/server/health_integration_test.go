//go:build integration

package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/config"
	"github.com/hakimyu/shlink-go/internal/server"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

func TestHealthWithDatabase(t *testing.T) {
	db := testutil.NewPostgres(t)
	app := server.New(config.Config{DefaultDomain: "s.test", CORSAllowedOrigins: []string{"*"}, CORSMaxAge: 3600}, db, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	req := httptest.NewRequest(http.MethodGet, "/rest/health", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/health+json", rec.Header().Get("Content-Type"))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "pass", payload["status"])
	require.Equal(t, "5.1.7", payload["version"])
	require.NotEmpty(t, rec.Header().Get("X-Request-Id"))
}
