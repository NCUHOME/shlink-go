//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NCUnix/shlink-go/internal/problem"
)

type requestOptions struct {
	APIKey     string
	Body       any
	Headers    map[string]string
	Host       string
	RemoteAddr string
}

func performRequest(t *testing.T, handler http.Handler, method, path string, opts requestOptions) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if opts.Body == nil {
		body = bytes.NewReader(nil)
	} else if raw, ok := opts.Body.(string); ok {
		body = bytes.NewReader([]byte(raw))
	} else {
		payload, err := json.Marshal(opts.Body)
		require.NoError(t, err)
		body = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, body)
	req.Host = "s.test"
	if opts.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if opts.APIKey != "" {
		req.Header.Set("X-Api-Key", opts.APIKey)
	}
	if opts.Host != "" {
		req.Host = opts.Host
	}
	if opts.RemoteAddr != "" {
		req.RemoteAddr = opts.RemoteAddr
	}
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var payload T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), rec.Body.String())
	return payload
}

func requireProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, typ string) problem.Problem {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
	return decodeBody[problem.Problem](t, rec)
}

func assertPagination(t *testing.T, got struct {
	CurrentPage        int `json:"currentPage"`
	PagesCount         int `json:"pagesCount"`
	ItemsPerPage       int `json:"itemsPerPage"`
	ItemsInCurrentPage int `json:"itemsInCurrentPage"`
	TotalItems         int `json:"totalItems"`
}, current, pages, perPage, inPage, total int) {
	t.Helper()
	require.Equal(t, current, got.CurrentPage)
	require.Equal(t, pages, got.PagesCount)
	require.Equal(t, perPage, got.ItemsPerPage)
	require.Equal(t, inPage, got.ItemsInCurrentPage)
	require.Equal(t, total, got.TotalItems)
}
