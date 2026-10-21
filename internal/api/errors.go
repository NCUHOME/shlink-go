package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hakimyu/shlink-go/internal/problem"
)

func internalProblem(ctx context.Context, err error) *problem.Problem {
	slog.ErrorContext(ctx, "request_failed", "error", err)
	return problem.New(
		http.StatusInternalServerError,
		"https://shlink.io/api/error/internal",
		"Internal error",
		"An unexpected error occurred.",
	)
}
