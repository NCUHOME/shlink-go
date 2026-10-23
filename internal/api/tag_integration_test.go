//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/page"
	"github.com/NCUnix/shlink-go/internal/problem"
	"github.com/NCUnix/shlink-go/internal/tag"
	"github.com/NCUnix/shlink-go/internal/testutil"
)

type tagListPayload struct {
	Tags struct {
		Data       []string        `json:"data"`
		Pagination page.Pagination `json:"pagination"`
	} `json:"tags"`
}

type tagStatsPayload struct {
	Tags struct {
		Data       []tag.Stat      `json:"data"`
		Pagination page.Pagination `json:"pagination"`
	} `json:"tags"`
}

func TestListTagsMatchesUpstreamCases(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for _, tc := range []struct {
		name       string
		apiKey     string
		query      string
		data       []string
		current    int
		pages      int
		perPage    int
		inPage     int
		totalItems int
	}{
		{"admin", "valid_api_key", "", []string{"bar", "baz", "foo"}, 1, 1, 3, 3, 3},
		{"admin pagination", "valid_api_key", "?page=2&itemsPerPage=2", []string{"foo"}, 2, 2, 2, 1, 3},
		{"author", "author_api_key", "", []string{"bar", "foo"}, 1, 1, 2, 2, 2},
		{"domain", "domain_api_key", "", []string{"foo"}, 1, 1, 1, 1, 1},
		{"search", "valid_api_key", "?searchTerm=ba", []string{"bar", "baz"}, 1, 1, 2, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/tags"+tc.query, requestOptions{APIKey: tc.apiKey})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			payload := decodeBody[tagListPayload](t, rec)
			require.Equal(t, tc.data, payload.Tags.Data)
			require.Equal(t, page.Pagination{
				CurrentPage:        tc.current,
				PagesCount:         tc.pages,
				ItemsPerPage:       tc.perPage,
				ItemsInCurrentPage: tc.inPage,
				TotalItems:         tc.totalItems,
			}, payload.Tags.Pagination)
		})
	}
}

func TestTagsStatsMatchesUpstreamCases(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for _, tc := range []struct {
		name   string
		apiKey string
		query  string
		data   []tag.Stat
		paging page.Pagination
	}{
		{
			name: "admin", apiKey: "valid_api_key",
			data: []tag.Stat{
				{Tag: "bar", ShortURLsCount: 1, VisitsSummary: tag.VisitSummary{Total: 2, NonBots: 1, Bots: 1}},
				{Tag: "baz", ShortURLsCount: 0, VisitsSummary: tag.VisitSummary{}},
				{Tag: "foo", ShortURLsCount: 3, VisitsSummary: tag.VisitSummary{Total: 5, NonBots: 4, Bots: 1}},
			},
			paging: page.Pagination{CurrentPage: 1, PagesCount: 1, ItemsPerPage: 3, ItemsInCurrentPage: 3, TotalItems: 3},
		},
		{
			name: "admin pagination", apiKey: "valid_api_key", query: "?page=1&itemsPerPage=2",
			data: []tag.Stat{
				{Tag: "bar", ShortURLsCount: 1, VisitsSummary: tag.VisitSummary{Total: 2, NonBots: 1, Bots: 1}},
				{Tag: "baz", ShortURLsCount: 0, VisitsSummary: tag.VisitSummary{}},
			},
			paging: page.Pagination{CurrentPage: 1, PagesCount: 2, ItemsPerPage: 2, ItemsInCurrentPage: 2, TotalItems: 3},
		},
		{
			name: "author", apiKey: "author_api_key",
			data: []tag.Stat{
				{Tag: "bar", ShortURLsCount: 1, VisitsSummary: tag.VisitSummary{Total: 2, NonBots: 1, Bots: 1}},
				{Tag: "foo", ShortURLsCount: 2, VisitsSummary: tag.VisitSummary{Total: 5, NonBots: 4, Bots: 1}},
			},
			paging: page.Pagination{CurrentPage: 1, PagesCount: 1, ItemsPerPage: 2, ItemsInCurrentPage: 2, TotalItems: 2},
		},
		{
			name: "author pagination", apiKey: "author_api_key", query: "?page=2&itemsPerPage=1",
			data: []tag.Stat{
				{Tag: "foo", ShortURLsCount: 2, VisitsSummary: tag.VisitSummary{Total: 5, NonBots: 4, Bots: 1}},
			},
			paging: page.Pagination{CurrentPage: 2, PagesCount: 2, ItemsPerPage: 1, ItemsInCurrentPage: 1, TotalItems: 2},
		},
		{
			name: "domain", apiKey: "domain_api_key",
			data: []tag.Stat{
				{Tag: "foo", ShortURLsCount: 1, VisitsSummary: tag.VisitSummary{}},
			},
			paging: page.Pagination{CurrentPage: 1, PagesCount: 1, ItemsPerPage: 1, ItemsInCurrentPage: 1, TotalItems: 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/tags/stats"+tc.query, requestOptions{APIKey: tc.apiKey})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			payload := decodeBody[tagStatsPayload](t, rec)
			require.Equal(t, tc.data, payload.Tags.Data)
			require.Equal(t, tc.paging, payload.Tags.Pagination)
		})
	}
}

func TestRenameTag(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for _, apiKey := range []string{"author_api_key", "domain_api_key"} {
		rec := performRequest(t, app.Handler(), http.MethodPut, "/rest/v3/tags", requestOptions{
			APIKey: apiKey,
			Body:   map[string]any{"oldName": "foo", "newName": "foo_renamed"},
		})
		payload := requireProblem(t, rec, http.StatusForbidden, problem.TypeForbiddenTagOperation)
		require.Equal(t, "You are not allowed to rename tags", payload.Detail)
	}

	for _, body := range []map[string]any{
		{},
		{"oldName": "foo"},
		{"newName": "foo"},
	} {
		rec := performRequest(t, app.Handler(), http.MethodPut, "/rest/v3/tags", requestOptions{APIKey: "valid_api_key", Body: body})
		requireProblem(t, rec, http.StatusBadRequest, problem.TypeInvalidData)
	}

	rec := performRequest(t, app.Handler(), http.MethodPut, "/rest/v3/tags", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"oldName": "invalid_tag", "newName": "foo"},
	})
	payload := requireProblem(t, rec, http.StatusNotFound, problem.TypeTagNotFound)
	require.Equal(t, `Tag with name "invalid_tag" could not be found`, payload.Detail)

	rec = performRequest(t, app.Handler(), http.MethodPut, "/rest/v3/tags", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"oldName": "foo", "newName": "bar"},
	})
	payload = requireProblem(t, rec, http.StatusConflict, problem.TypeTagConflict)
	require.Equal(t, "You cannot rename tag foo to bar, because it already exists", payload.Detail)

	rec = performRequest(t, app.Handler(), http.MethodPut, "/rest/v3/tags", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"oldName": "foo", "newName": "foo"},
	})
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = performRequest(t, app.Handler(), http.MethodPut, "/rest/v3/tags", requestOptions{
		APIKey: "valid_api_key",
		Body:   map[string]any{"oldName": "foo", "newName": "foo_renamed"},
	})
	require.Equal(t, http.StatusNoContent, rec.Code)
	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/tags", requestOptions{APIKey: "valid_api_key"})
	require.Contains(t, decodeBody[tagListPayload](t, rec).Tags.Data, "foo_renamed")
}

func TestDeleteTags(t *testing.T) {
	db := testutil.NewPostgres(t)
	fixtures := testutil.SeedBase(t, db)
	testutil.SeedShortURLs(t, db, fixtures)
	app := testutil.NewApp(t, db)

	for _, apiKey := range []string{"author_api_key", "domain_api_key"} {
		rec := performRequest(t, app.Handler(), http.MethodDelete, "/rest/v3/tags?tags[]=foo", requestOptions{APIKey: apiKey})
		payload := requireProblem(t, rec, http.StatusForbidden, problem.TypeForbiddenTagOperation)
		require.Equal(t, "You are not allowed to delete tags", payload.Detail)
	}

	rec := performRequest(t, app.Handler(), http.MethodDelete, "/rest/v3/tags?tags[]=foo&tags[]=baz", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = performRequest(t, app.Handler(), http.MethodGet, "/rest/v3/tags", requestOptions{APIKey: "valid_api_key"})
	require.Equal(t, []string{"bar"}, decodeBody[tagListPayload](t, rec).Tags.Data)
}
