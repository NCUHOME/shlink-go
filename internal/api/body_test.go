package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/hakimyu/shlink-go/internal/problem"
)

func TestDecodeJSONReturnsProblemForMalformedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/test", func(c *gin.Context) {
		var payload map[string]any
		if p := DecodeJSON(c, &payload); p != nil {
			problem.Write(c, p)
			return
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, problem.TypeMalformedRequestBody, payload["type"])
	require.Equal(t, "Provided request does not contain a valid JSON body.", payload["detail"])
}
