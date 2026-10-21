package query

import (
	"context"
	"net/url"

	"github.com/gin-gonic/gin"
)

type arraysContextKey struct{}

var arrayKeys = []string{"tags", "excludeTags"}

func ArraysMiddleware() func(*gin.Context) {
	return func(c *gin.Context) {
		values := c.Request.URL.Query()
		parsed := make(map[string][]string, len(arrayKeys))
		for _, key := range arrayKeys {
			parsed[key] = readArray(values, key)
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), arraysContextKey{}, parsed))
		c.Next()
	}
}

func FromContext(ctx context.Context, key string) []string {
	values, _ := ctx.Value(arraysContextKey{}).(map[string][]string)
	return values[key]
}

func readArray(values url.Values, key string) []string {
	if vals, ok := values[key+"[]"]; ok {
		return vals
	}
	return values[key]
}
