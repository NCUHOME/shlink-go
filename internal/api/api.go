package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

func NewHuma(engine *gin.Engine) huma.API {
	cfg := huma.DefaultConfig("Shlink Go API", "5.1.7")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	// Shlink v3 responses do not include Huma's $schema field or Link header.
	cfg.CreateHooks = nil
	return humagin.New(engine, cfg)
}
