package query

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestArraysMiddlewareReadsBracketAndRepeatedParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ArraysMiddleware())
	r.GET("/test", func(c *gin.Context) {
		require.Equal(t, []string{"foo", "bar"}, FromContext(c.Request.Context(), "tags"))
		require.Equal(t, []string{"baz"}, FromContext(c.Request.Context(), "excludeTags"))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test?tags[]=foo&tags[]=bar&excludeTags=baz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}
