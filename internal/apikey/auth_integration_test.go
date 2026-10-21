//go:build integration

package apikey_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/apikey"
	"github.com/hakimyu/shlink-go/internal/problem"
	"github.com/hakimyu/shlink-go/internal/testutil"
)

func TestAuthenticateFixtures(t *testing.T) {
	db := testutil.NewPostgres(t)
	f := testutil.SeedBase(t, db)
	auth := apikey.NewAuthenticator(db.DB)

	t.Run("admin key", func(t *testing.T) {
		p, err := auth.Authenticate(t.Context(), "valid_api_key")
		require.NoError(t, err)
		require.True(t, p.Scope.IsAdmin)
	})
	t.Run("authored key", func(t *testing.T) {
		p, err := auth.Authenticate(t.Context(), "author_api_key")
		require.NoError(t, err)
		require.False(t, p.Scope.IsAdmin)
		require.NotNil(t, p.Scope.AuthorAPIKeyID)
		require.Equal(t, f.APIKeyIDs["author_api_key"], *p.Scope.AuthorAPIKeyID)
	})
	t.Run("domain key", func(t *testing.T) {
		p, err := auth.Authenticate(t.Context(), "domain_api_key")
		require.NoError(t, err)
		require.NotNil(t, p.Scope.DomainID)
		require.Equal(t, f.ExampleDomainID, *p.Scope.DomainID)
	})
	t.Run("no orphan key", func(t *testing.T) {
		p, err := auth.Authenticate(t.Context(), "no_orphans_api_key")
		require.NoError(t, err)
		require.True(t, p.Scope.DenyOrphan)
	})
	for _, raw := range []string{"missing", "disabled_api_key", "expired_api_key"} {
		t.Run("invalid "+raw, func(t *testing.T) {
			_, err := auth.Authenticate(t.Context(), raw)
			require.ErrorIs(t, err, apikey.ErrInvalidKey)
		})
	}
}

func TestMiddlewareRejectsMissingAndInvalidKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewPostgres(t)
	testutil.SeedBase(t, db)
	auth := apikey.NewAuthenticator(db.DB)

	r := gin.New()
	r.Use(apikey.Middleware(auth))
	r.GET("/rest/v3/test", func(c *gin.Context) {
		p, ok := apikey.FromGin(c)
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{"name": p.Name})
	})

	for _, tc := range []struct {
		name string
		key  string
		typ  string
	}{
		{"missing", "", problem.TypeMissingAuthentication},
		{"invalid", "nope", problem.TypeInvalidAPIKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/rest/v3/test", nil)
			if tc.key != "" {
				req.Header.Set("X-Api-Key", tc.key)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			require.Equal(t, tc.typ, payload["type"])
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/rest/v3/test", nil)
	req.Header.Set("X-Api-Key", "valid_api_key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "valid_api_key")
}
