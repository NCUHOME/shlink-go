package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/NCUnix/shlink-go/internal/apikey"
	"github.com/NCUnix/shlink-go/internal/problem"
	"github.com/NCUnix/shlink-go/internal/redirectrule"
	"github.com/NCUnix/shlink-go/internal/shorturl"
)

type redirectRulesOutput struct {
	Body redirectrule.Set
}

type setRedirectRulesInput struct {
	ShortCode string `path:"shortCode"`
	Domain    string `query:"domain"`
	RawBody   []byte `contentType:"application/json"`
}

type setRedirectRulesBody struct {
	RedirectRules []redirectrule.Rule `json:"redirectRules"`
}

func RegisterRedirectRules(api huma.API, shortURLs *shorturl.Service, rules *redirectrule.Service) {
	huma.Register(api, huma.Operation{
		OperationID:        "listShortUrlRedirectRules",
		Method:             http.MethodGet,
		Path:               "/rest/v3/short-urls/{shortCode}/redirect-rules",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *shortCodeInput) (*redirectRulesOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		resolved, err := shortURLs.ResolveForScope(ctx, principal.Scope, input.ShortCode, stringPtr(input.Domain))
		if err != nil {
			return nil, mapShortURLError(ctx, err, input.ShortCode)
		}
		set, err := rules.Get(ctx, resolved.ID, resolved.LongURL)
		if err != nil {
			return nil, internalProblem(ctx, err)
		}
		return &redirectRulesOutput{Body: *set}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "setShortUrlRedirectRules",
		Method:             http.MethodPost,
		Path:               "/rest/v3/short-urls/{shortCode}/redirect-rules",
		SkipValidateBody:   true,
		SkipValidateParams: true,
	}, func(ctx context.Context, input *setRedirectRulesInput) (*redirectRulesOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		resolved, err := shortURLs.ResolveForScope(ctx, principal.Scope, input.ShortCode, stringPtr(input.Domain))
		if err != nil {
			return nil, mapShortURLError(ctx, err, input.ShortCode)
		}
		var body setRedirectRulesBody
		if p := DecodeJSONBytes(input.RawBody, &body); p != nil {
			return nil, p
		}
		if body.RedirectRules == nil {
			return nil, problem.InvalidData("redirectRules")
		}
		set, err := rules.Replace(ctx, resolved.ID, resolved.LongURL, body.RedirectRules)
		if err != nil {
			var validation *redirectrule.ValidationError
			if errors.As(err, &validation) {
				return nil, problem.InvalidData(validation.Elements...)
			}
			return nil, internalProblem(ctx, err)
		}
		return &redirectRulesOutput{Body: *set}, nil
	})
}
