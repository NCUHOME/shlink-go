package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hakimyu/shlink-go/internal/apikey"
	"github.com/hakimyu/shlink-go/internal/domain"
	"github.com/hakimyu/shlink-go/internal/problem"
	"github.com/hakimyu/shlink-go/internal/shorturl"
	"github.com/hakimyu/shlink-go/internal/tag"
	"github.com/hakimyu/shlink-go/internal/visit"
)

type visitsInput struct {
	Page         string `query:"page"`
	ItemsPerPage string `query:"itemsPerPage"`
	StartDate    string `query:"startDate"`
	EndDate      string `query:"endDate"`
	ExcludeBots  string `query:"excludeBots"`
	OrderBy      string `query:"orderBy"`
	Domain       string `query:"domain"`
	Type         string `query:"type"`
}

type shortURLVisitsInput struct {
	ShortCode    string `path:"shortCode"`
	Domain       string `query:"domain"`
	Page         string `query:"page"`
	ItemsPerPage string `query:"itemsPerPage"`
	StartDate    string `query:"startDate"`
	EndDate      string `query:"endDate"`
	ExcludeBots  string `query:"excludeBots"`
	OrderBy      string `query:"orderBy"`
}

type tagVisitsInput struct {
	Tag          string `path:"tag"`
	Page         string `query:"page"`
	ItemsPerPage string `query:"itemsPerPage"`
	StartDate    string `query:"startDate"`
	EndDate      string `query:"endDate"`
	ExcludeBots  string `query:"excludeBots"`
	OrderBy      string `query:"orderBy"`
	Domain       string `query:"domain"`
}

type domainVisitsInput struct {
	Domain       string `path:"domain"`
	Page         string `query:"page"`
	ItemsPerPage string `query:"itemsPerPage"`
	StartDate    string `query:"startDate"`
	EndDate      string `query:"endDate"`
	ExcludeBots  string `query:"excludeBots"`
	OrderBy      string `query:"orderBy"`
}

type visitsOutput struct {
	Body struct {
		Visits visit.ListResult[visit.Visit] `json:"visits"`
	}
}

type orphanVisitsOutput struct {
	Body struct {
		Visits visit.ListResult[visit.OrphanVisit] `json:"visits"`
	}
}

type globalVisitsOutput struct {
	Body struct {
		Visits visit.Stats `json:"visits"`
	}
}

type deletedVisitsOutput struct {
	Body struct {
		DeletedVisits int64 `json:"deletedVisits"`
	}
}

func RegisterVisits(api huma.API, shortURLs *shorturl.Service, tags *tag.Service, domains *domain.Repository, visits *visit.Service) {
	huma.Register(api, huma.Operation{
		OperationID:        "globalVisits",
		Method:             http.MethodGet,
		Path:               "/rest/v3/visits",
		SkipValidateParams: true,
	}, func(ctx context.Context, _ *struct{}) (*globalVisitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		stats, err := visits.GlobalStats(ctx, principal.Scope)
		if err != nil {
			return nil, internalProblem(ctx, err)
		}
		out := &globalVisitsOutput{}
		out.Body.Visits = *stats
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "shortUrlVisits",
		Method:             http.MethodGet,
		Path:               "/rest/v3/short-urls/{shortCode}/visits",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *shortURLVisitsInput) (*visitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		params, p := parseVisitParams(visitsInput{
			Page: input.Page, ItemsPerPage: input.ItemsPerPage, StartDate: input.StartDate,
			EndDate: input.EndDate, ExcludeBots: input.ExcludeBots, OrderBy: input.OrderBy,
		})
		if p != nil {
			return nil, p
		}
		resolved, err := shortURLs.ResolveForScope(ctx, principal.Scope, input.ShortCode, stringPtr(input.Domain))
		if err != nil {
			return nil, mapShortURLError(ctx, err, input.ShortCode)
		}
		result, err := visits.ListForShortURL(ctx, principal.Scope, resolved.ID, *params)
		if err != nil {
			return nil, mapVisitError(ctx, err)
		}
		out := &visitsOutput{}
		out.Body.Visits = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "tagVisits",
		Method:             http.MethodGet,
		Path:               "/rest/v3/tags/{tag}/visits",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *tagVisitsInput) (*visitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		params, p := parseVisitParams(visitsInput{
			Page: input.Page, ItemsPerPage: input.ItemsPerPage, StartDate: input.StartDate,
			EndDate: input.EndDate, ExcludeBots: input.ExcludeBots, OrderBy: input.OrderBy, Domain: input.Domain,
		})
		if p != nil {
			return nil, p
		}
		exists, err := tags.ExistsVisible(ctx, principal.Scope, input.Tag)
		if err != nil {
			return nil, internalProblem(ctx, err)
		}
		if !exists {
			return nil, problem.TagNotFound(input.Tag)
		}
		result, err := visits.ListForTag(ctx, principal.Scope, tag.Normalize(input.Tag), *params)
		if err != nil {
			return nil, mapVisitError(ctx, err)
		}
		out := &visitsOutput{}
		out.Body.Visits = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "domainVisits",
		Method:             http.MethodGet,
		Path:               "/rest/v3/domains/{domain}/visits",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *domainVisitsInput) (*visitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		params, p := parseVisitParams(visitsInput{
			Page: input.Page, ItemsPerPage: input.ItemsPerPage, StartDate: input.StartDate,
			EndDate: input.EndDate, ExcludeBots: input.ExcludeBots, OrderBy: input.OrderBy,
		})
		if p != nil {
			return nil, p
		}
		if p := ensureVisibleDomain(ctx, domains, principal.Scope, input.Domain); p != nil {
			return nil, p
		}
		result, err := visits.ListForDomain(ctx, principal.Scope, input.Domain, *params)
		if err != nil {
			return nil, mapVisitError(ctx, err)
		}
		out := &visitsOutput{}
		out.Body.Visits = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "nonOrphanVisits",
		Method:             http.MethodGet,
		Path:               "/rest/v3/visits/non-orphan",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *visitsInput) (*visitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		params, p := parseVisitParams(*input)
		if p != nil {
			return nil, p
		}
		result, err := visits.ListNonOrphan(ctx, principal.Scope, *params)
		if err != nil {
			return nil, mapVisitError(ctx, err)
		}
		out := &visitsOutput{}
		out.Body.Visits = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "orphanVisits",
		Method:             http.MethodGet,
		Path:               "/rest/v3/visits/orphan",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *visitsInput) (*orphanVisitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		params, p := parseVisitParams(*input)
		if p != nil {
			return nil, p
		}
		result, err := visits.ListOrphan(ctx, principal.Scope, *params)
		if err != nil {
			return nil, mapVisitError(ctx, err)
		}
		out := &orphanVisitsOutput{}
		out.Body.Visits = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "deleteShortUrlVisits",
		Method:             http.MethodDelete,
		Path:               "/rest/v3/short-urls/{shortCode}/visits",
		SkipValidateParams: true,
	}, func(ctx context.Context, input *shortCodeInput) (*deletedVisitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		resolved, err := shortURLs.ResolveForScope(ctx, principal.Scope, input.ShortCode, stringPtr(input.Domain))
		if err != nil {
			return nil, mapShortURLError(ctx, err, input.ShortCode)
		}
		deleted, err := visits.DeleteShortURLVisits(ctx, resolved.ID)
		if err != nil {
			return nil, internalProblem(ctx, err)
		}
		out := &deletedVisitsOutput{}
		out.Body.DeletedVisits = deleted
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:        "deleteOrphanVisits",
		Method:             http.MethodDelete,
		Path:               "/rest/v3/visits/orphan",
		SkipValidateParams: true,
	}, func(ctx context.Context, _ *struct{}) (*deletedVisitsOutput, error) {
		principal, ok := apikey.FromContext(ctx)
		if !ok {
			return nil, problem.MissingAuthentication()
		}
		deleted, err := visits.DeleteOrphanVisits(ctx, principal.Scope)
		if err != nil {
			return nil, internalProblem(ctx, err)
		}
		out := &deletedVisitsOutput{}
		out.Body.DeletedVisits = deleted
		return out, nil
	})
}

func parseVisitParams(input visitsInput) (*visit.ListParams, *problem.Problem) {
	params := &visit.ListParams{
		Page: 1, OrderBy: input.OrderBy, Domain: input.Domain, Type: input.Type,
	}
	invalid := make([]string, 0)
	if input.Page != "" {
		value, err := strconv.Atoi(input.Page)
		if err != nil || value <= 0 {
			invalid = append(invalid, "page")
		} else {
			params.Page = value
		}
	}
	if input.ItemsPerPage != "" {
		value, err := strconv.Atoi(input.ItemsPerPage)
		if err != nil || value <= 0 {
			invalid = append(invalid, "itemsPerPage")
		} else {
			params.ItemsPerPage = value
		}
	}
	start, p := parseOptionalTime(stringPtr(input.StartDate), "startDate")
	if p != nil {
		invalid = append(invalid, "startDate")
	} else {
		params.StartDate = start
	}
	end, p := parseOptionalTime(stringPtr(input.EndDate), "endDate")
	if p != nil {
		invalid = append(invalid, "endDate")
	} else {
		params.EndDate = end
	}
	if input.ExcludeBots != "" {
		value, err := strconv.ParseBool(input.ExcludeBots)
		if err != nil {
			invalid = append(invalid, "excludeBots")
		} else {
			params.ExcludeBots = value
		}
	}
	if len(invalid) > 0 {
		return nil, problem.InvalidData(invalid...)
	}
	return params, nil
}

func ensureVisibleDomain(ctx context.Context, repo *domain.Repository, scope apikey.Scope, requested string) *problem.Problem {
	domainName := requested
	if strings.EqualFold(domainName, "DEFAULT") {
		if _, err := repo.GetDefault(ctx); err != nil {
			return problem.DomainNotFound(requested)
		}
		// The DEFAULT keyword always resolves to the configured default domain,
		// even for domain-scoped keys. The visit query below still applies the
		// key scope, so such a key simply gets an empty list.
		return nil
	}
	visible, err := repo.ListVisible(ctx, scope)
	if err != nil {
		return internalProblem(ctx, err)
	}
	for _, item := range visible {
		if strings.EqualFold(item.Authority, domainName) {
			return nil
		}
	}
	return problem.DomainNotFound(domainName)
}

func mapVisitError(ctx context.Context, err error) *problem.Problem {
	var validation *visit.ValidationError
	if errors.As(err, &validation) {
		return problem.InvalidData(validation.Elements...)
	}
	return internalProblem(ctx, err)
}
