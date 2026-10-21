package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImplicitOptionsReturnsAllowedMethods(t *testing.T) {
	app := testServer()

	for _, tc := range []struct {
		path  string
		allow string
	}{
		{"/rest/v3/short-urls", "GET,POST"},
		{"/rest/v3/tags", "GET,DELETE,PUT"},
		{"/rest/v3/health", "GET"},
		{"/rest/v3/domains", "GET"},
		{"/rest/v3/domains/redirects", "PATCH"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, tc.path, nil)
			rec := httptest.NewRecorder()
			app.Handler().ServeHTTP(rec, req)

			require.Equal(t, http.StatusNoContent, rec.Code)
			require.Empty(t, rec.Body.String())
			require.Equal(t, tc.allow, rec.Header().Get("Allow"))
			require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}
