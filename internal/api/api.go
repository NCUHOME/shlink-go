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
	return humagin.New(engine, cfg)
}
