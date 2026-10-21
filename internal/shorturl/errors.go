package shorturl

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Elements []string
}

func (e *ValidationError) Error() string {
	return "invalid elements: " + strings.Join(e.Elements, ", ")
}

type NotFoundError struct {
	ShortCode string
	Domain    string
}

func (e *NotFoundError) Error() string {
	if e.Domain == "" {
		return fmt.Sprintf("short URL %q not found", e.ShortCode)
	}
	return fmt.Sprintf("short URL %q not found for domain %q", e.ShortCode, e.Domain)
}

type ConflictError struct {
	Slug   string
	Domain string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("slug %q is already in use", e.Slug)
}
