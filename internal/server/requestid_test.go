package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestIDGeneratedAndPreserved(t *testing.T) {
	app := testServer()

	req := httptest.NewRequest(http.MethodGet, "/rest/health", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	require.NotEmpty(t, rec.Header().Get("X-Request-Id"))

	req = httptest.NewRequest(http.MethodGet, "/rest/health", nil)
	req.Header.Set("X-Request-Id", "foobar")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	require.Equal(t, "foobar", rec.Header().Get("X-Request-Id"))
}
