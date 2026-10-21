package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hakimyu/shlink-go/internal/problem"
)

func RegisterMercure(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:        "mercureInfo",
		Method:             http.MethodGet,
		Path:               "/rest/v3/mercure-info",
		SkipValidateParams: true,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, problem.MercureNotConfigured()
	})
}
