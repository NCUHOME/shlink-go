package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hakimyu/shlink-go/internal/config"
)

func corsMiddleware(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowedOrigin, ok := allowedOrigin(cfg.CORSAllowedOrigins, origin)
		if !ok {
			c.Next()
			return
		}
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		if allowedOrigin != "*" {
			c.Header("Vary", "Origin")
		}
		if cfg.CORSAllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			methods := allowedMethods(c.Request.URL.Path)
			c.Header("Access-Control-Allow-Methods", methods)
			c.Header("Allow", methods)
			headers := c.GetHeader("Access-Control-Request-Headers")
			if headers == "" {
				headers = "X-Api-Key, Content-Type"
			}
			c.Header("Access-Control-Allow-Headers", headers)
			c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.CORSMaxAge))
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func allowedOrigin(origins []string, origin string) (string, bool) {
	for _, allowed := range origins {
		if allowed == "*" {
			return "*", true
		}
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return origin, true
		}
	}
	return "", false
}

func allowedMethods(path string) string {
	if index := strings.Index(path, "/rest/v"); index >= 0 {
		rest := path[index:]
		parts := strings.SplitN(rest, "/", 4)
		if len(parts) == 4 {
			path = "/" + parts[3]
		}
	}
	switch path {
	case "/rest/health", "/rest/v3/health", "/health":
		return "GET"
	case "/short-urls":
		return "GET,POST"
	case "/tags":
		return "GET,DELETE,PUT"
	case "/short-urls/{shortCode}", "/short-urls/{shortCode}/visits":
		return "GET,PATCH,DELETE"
	case "/domains":
		return "GET"
	case "/domains/redirects":
		return "PATCH"
	case "/visits", "/visits/orphan", "/visits/non-orphan":
		return "GET,DELETE"
	default:
		return "GET,POST,PUT,PATCH,DELETE"
	}
}
