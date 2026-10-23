//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/testutil"
)

func TestV3ResponsesMatchOpenAPIContract(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "shlink", "docs", "swagger", "swagger.json")
	if _, err := os.Stat(specPath); err != nil {
		t.Skipf("upstream OpenAPI spec is not available: %v", err)
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromFile(specPath)
	require.NoError(t, err)

	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	shortURLs := testutil.SeedShortURLs(t, db, fixtures)
	testutil.SeedVisits(t, db, fixtures, shortURLs)
	testutil.SeedRedirectRules(t, db, shortURLs["1/def456"])
	app := testutil.NewApp(t, db)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
	}{
		{"list short urls", http.MethodGet, "/rest/v3/short-urls", nil, http.StatusOK},
		{"create short url", http.MethodPost, "/rest/v3/short-urls", map[string]any{"longUrl": "https://example.com/contract", "customSlug": "contract"}, http.StatusOK},
		{"shorten short url", http.MethodGet, "/rest/v3/short-urls/shorten?longUrl=https%3A%2F%2Fexample.com%2Fshorten&apiKey=valid_api_key", nil, http.StatusOK},
		{"get short url", http.MethodGet, "/rest/v3/short-urls/abc123", nil, http.StatusOK},
		{"edit short url", http.MethodPatch, "/rest/v3/short-urls/abc123", map[string]any{"title": "contract title"}, http.StatusOK},
		{"list redirect rules", http.MethodGet, "/rest/v3/short-urls/def456/redirect-rules", nil, http.StatusOK},
		{"set redirect rules", http.MethodPost, "/rest/v3/short-urls/abc123/redirect-rules", map[string]any{"redirectRules": []any{}}, http.StatusOK},
		{"list tags", http.MethodGet, "/rest/v3/tags", nil, http.StatusOK},
		{"tags stats", http.MethodGet, "/rest/v3/tags/stats", nil, http.StatusOK},
		{"list domains", http.MethodGet, "/rest/v3/domains", nil, http.StatusOK},
		{"domain redirects", http.MethodPatch, "/rest/v3/domains/redirects", map[string]any{"domain": "example.com", "regular404Redirect": "https://example.com/404"}, http.StatusOK},
		{"global visits", http.MethodGet, "/rest/v3/visits", nil, http.StatusOK},
		{"short url visits", http.MethodGet, "/rest/v3/short-urls/abc123/visits", nil, http.StatusOK},
		{"tag visits", http.MethodGet, "/rest/v3/tags/foo/visits", nil, http.StatusOK},
		{"domain visits", http.MethodGet, "/rest/v3/domains/DEFAULT/visits", nil, http.StatusOK},
		{"orphan visits", http.MethodGet, "/rest/v3/visits/orphan", nil, http.StatusOK},
		{"non orphan visits", http.MethodGet, "/rest/v3/visits/non-orphan", nil, http.StatusOK},
		{"delete short url visits", http.MethodDelete, "/rest/v3/short-urls/abc123/visits", nil, http.StatusOK},
		{"delete orphan visits", http.MethodDelete, "/rest/v3/visits/orphan", nil, http.StatusOK},
		{"rename tag", http.MethodPut, "/rest/v3/tags", map[string]any{"oldName": "foo", "newName": "contract-tag"}, http.StatusNoContent},
		{"delete tag", http.MethodDelete, "/rest/v3/tags?tags[]=baz", nil, http.StatusNoContent},
		{"delete short url", http.MethodDelete, "/rest/v3/short-urls/contract", nil, http.StatusNoContent},
		{"mercure info", http.MethodGet, "/rest/v3/mercure-info", nil, http.StatusNotImplemented},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), tc.method, tc.path, requestOptions{
				APIKey: "valid_api_key",
				Body:   tc.body,
			})
			require.Equal(t, tc.status, rec.Code, rec.Body.String())

			operation := findOperation(t, spec, tc.method, tc.path)
			require.NotNil(t, operation, "operation not found in OpenAPI spec")
			response := operation.Responses.Status(tc.status)
			require.NotNil(t, response, "status %d not declared for %s %s", tc.status, tc.method, tc.path)
			if rec.Body.Len() == 0 {
				return
			}
			mediaType := strings.TrimSpace(strings.Split(rec.Header().Get("Content-Type"), ";")[0])
			content := response.Value.Content[mediaType]
			if content == nil {
				// Some upstream error responses are documented only as
				// application/json, while Shlink emits problem+json.
				content = response.Value.Content["application/json"]
			}
			require.NotNil(t, content, "no schema for media type %q", mediaType)
			require.NotNil(t, content.Schema, "no schema reference for media type %q", mediaType)
			require.NotNil(t, content.Schema.Value, "unresolved schema for media type %q", mediaType)
			var payload any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			normalizeNullableVisitLocations(payload)
			require.NoError(t, content.Schema.Value.VisitJSON(payload), "response does not match upstream schema")
		})
	}
}

// The upstream schema describes visitLocation as a non-nullable object, but
// Shlink itself returns null when GeoIP is disabled. Normalize that known
// discrepancy so the contract test still catches other field mismatches.
func normalizeNullableVisitLocations(value any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			normalizeNullableVisitLocations(item)
		}
	case map[string]any:
		if location, ok := typed["visitLocation"]; ok && location == nil {
			typed["visitLocation"] = map[string]any{}
		}
		for _, item := range typed {
			normalizeNullableVisitLocations(item)
		}
	}
}

func findOperation(t *testing.T, spec *openapi3.T, method, actualPath string) *openapi3.Operation {
	t.Helper()
	actualPath = strings.SplitN(actualPath, "?", 2)[0]
	placeholder := regexp.MustCompile(`\{[^}]+\}`)
	for template, item := range spec.Paths.Map() {
		if item == nil {
			continue
		}
		concrete := strings.ReplaceAll(template, "{version}", "3")
		pattern := "^" + placeholder.ReplaceAllString(concrete, `[^/]+`) + "$"
		matched, err := regexp.MatchString(pattern, actualPath)
		require.NoError(t, err)
		if !matched {
			continue
		}
		if operation := item.GetOperation(method); operation != nil {
			return operation
		}
	}
	return nil
}
