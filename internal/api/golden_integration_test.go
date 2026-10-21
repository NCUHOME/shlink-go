//go:build integration

package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/testutil"
)

func TestGoldenResponses(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	shortURLs := testutil.SeedShortURLs(t, db, fixtures)
	testutil.SeedVisits(t, db, fixtures, shortURLs)
	testutil.SeedRedirectRules(t, db, shortURLs["1/def456"])
	app := testutil.NewApp(t, db)

	for _, tc := range []struct {
		name   string
		path   string
		status int
	}{
		{"short-url", "/rest/v3/short-urls/abc123", http.StatusOK},
		{"tags", "/rest/v3/tags", http.StatusOK},
		{"domains", "/rest/v3/domains", http.StatusOK},
		{"orphan-visits", "/rest/v3/visits/orphan", http.StatusOK},
		{"redirect-rules", "/rest/v3/short-urls/def456/redirect-rules", http.StatusOK},
		{"short-url-not-found", "/rest/v3/short-urls/invalid", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), http.MethodGet, tc.path, requestOptions{APIKey: "valid_api_key"})
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			goldenPath := filepath.Join("testdata", "golden", tc.name+".json")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
				require.NoError(t, os.WriteFile(goldenPath, rec.Body.Bytes(), 0o644))
			}
			expected, err := os.ReadFile(goldenPath)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), rec.Body.String())
		})
	}
}
