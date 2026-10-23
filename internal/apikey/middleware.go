package apikey

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/NCUnix/shlink-go/internal/problem"
)

type principalContextKey struct{}

func Middleware(auth *Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions || !requiresAuth(c.Request.URL.Path) {
			c.Next()
			return
		}

		queryAuth := allowsQueryAPIKey(c.Request.URL.Path)
		raw := c.GetHeader("X-Api-Key")
		if queryAuth {
			raw = c.Query("apiKey")
		}
		if raw == "" {
			if queryAuth {
				problem.Abort(c, problem.MissingQueryAuthentication())
			} else {
				problem.Abort(c, problem.MissingAuthentication())
			}
			return
		}
		principal, err := auth.Authenticate(c.Request.Context(), raw)
		if err != nil {
			problem.Abort(c, problem.InvalidAPIKey())
			return
		}
		c.Set("api_key_name", principal.Name)
		ctx := context.WithValue(c.Request.Context(), principalContextKey{}, principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func requiresAuth(path string) bool {
	if !strings.HasPrefix(path, "/rest/") {
		return false
	}
	return path != "/rest/health" && path != "/rest/v3/health"
}

func allowsQueryAPIKey(path string) bool {
	return path == "/rest/v3/short-urls/shorten"
}

func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(*Principal)
	return p, ok
}

func FromGin(c *gin.Context) (*Principal, bool) {
	return FromContext(c.Request.Context())
}
