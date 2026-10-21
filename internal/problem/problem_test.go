package problem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInvalidDataProblem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	Abort(c, InvalidData("maxVisits", "validSince"))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/problem+json; charset=utf-8", rec.Header().Get("Content-Type"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, TypeInvalidData, payload["type"])
	require.Equal(t, "Invalid data", payload["title"])
	require.Equal(t, "Provided data is not valid", payload["detail"])
	require.Equal(t, float64(400), payload["status"])
	require.Equal(t, []any{"maxVisits", "validSince"}, payload["invalidElements"])
}

func TestProblemOmitsEmptyExtraFields(t *testing.T) {
	body := Encode(ShortURLNotFound("abc123", ""))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.NotContains(t, payload, "domain")
	require.Equal(t, "abc123", payload["shortCode"])
}
