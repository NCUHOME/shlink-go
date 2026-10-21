package problem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	TypeInvalidData             = "https://shlink.io/api/error/invalid-data"
	TypeNonUniqueSlug           = "https://shlink.io/api/error/non-unique-slug"
	TypeShortURLNotFound        = "https://shlink.io/api/error/short-url-not-found"
	TypeInvalidShortURLDeletion = "https://shlink.io/api/error/invalid-short-url-deletion"
	TypeDomainNotFound          = "https://shlink.io/api/error/domain-not-found"
	TypeForbiddenTagOperation   = "https://shlink.io/api/error/forbidden-tag-operation"
	TypeTagConflict             = "https://shlink.io/api/error/tag-conflict"
	TypeTagNotFound             = "https://shlink.io/api/error/tag-not-found"
	TypeMercureNotConfigured    = "https://shlink.io/api/error/mercure-not-configured"
	TypeMissingAuthentication   = "https://shlink.io/api/error/missing-authentication"
	TypeInvalidAPIKey           = "https://shlink.io/api/error/invalid-api-key"
	TypeNotFound                = "https://shlink.io/api/error/not-found"
	TypeMalformedRequestBody    = "https://shlink.io/api/error/malformed-request-body"
)

type Problem struct {
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	Status          int      `json:"status"`
	InvalidElements []string `json:"invalidElements,omitempty"`
	ShortCode       string   `json:"shortCode,omitempty"`
	Tag             string   `json:"tag,omitempty"`
	Domain          string   `json:"domain,omitempty"`
	Authority       string   `json:"authority,omitempty"`
	CustomSlug      string   `json:"customSlug,omitempty"`
}

func (p *Problem) Error() string { return p.Detail }

func (p *Problem) ContentType(string) string { return "application/problem+json" }

func (p *Problem) GetStatus() int { return p.Status }

func New(status int, typ, title, detail string) *Problem {
	return &Problem{Type: typ, Title: title, Detail: detail, Status: status}
}

func InvalidData(elements ...string) *Problem {
	return &Problem{
		Type:            TypeInvalidData,
		Title:           "Invalid data",
		Detail:          "Provided data is not valid",
		Status:          http.StatusBadRequest,
		InvalidElements: elements,
	}
}

func MalformedBody() *Problem {
	return New(
		http.StatusBadRequest,
		TypeMalformedRequestBody,
		"Malformed request body",
		"Provided request does not contain a valid JSON body.",
	)
}

func MissingAuthentication() *Problem {
	return New(
		http.StatusUnauthorized,
		TypeMissingAuthentication,
		"Invalid authorization",
		`Expected one of the following authentication headers, ["X-Api-Key"], but none were provided`,
	)
}

func MissingQueryAuthentication() *Problem {
	return New(
		http.StatusUnauthorized,
		TypeMissingAuthentication,
		"Invalid authorization",
		`Expected authentication to be provided in "apiKey" query param`,
	)
}

func InvalidAPIKey() *Problem {
	return New(
		http.StatusUnauthorized,
		TypeInvalidAPIKey,
		"Invalid API key",
		"Provided API key does not exist or is invalid.",
	)
}

func NotFound() *Problem {
	return New(http.StatusNotFound, TypeNotFound, "Not found", "The requested resource could not be found.")
}

func ShortURLNotFound(shortCode, domain string) *Problem {
	detail := fmt.Sprintf("No URL found with short code %q", shortCode)
	if domain != "" {
		detail = fmt.Sprintf("No URL found with short code %q for domain %q", shortCode, domain)
	}
	p := New(
		http.StatusNotFound,
		TypeShortURLNotFound,
		"Short URL not found",
		detail,
	)
	p.ShortCode = shortCode
	if domain != "" {
		p.Domain = domain
	}
	return p
}

func TagNotFound(tag string) *Problem {
	p := New(
		http.StatusNotFound,
		TypeTagNotFound,
		"Tag not found",
		fmt.Sprintf("Tag with name %q could not be found", tag),
	)
	p.Tag = tag
	return p
}

func DomainNotFound(authority string) *Problem {
	p := New(
		http.StatusNotFound,
		TypeDomainNotFound,
		"Domain not found",
		fmt.Sprintf("Domain with authority %q could not be found", authority),
	)
	p.Authority = authority
	return p
}

func ForbiddenTagOperation(operation string) *Problem {
	return New(
		http.StatusForbidden,
		TypeForbiddenTagOperation,
		"Forbidden tag operation",
		"You are not allowed to "+operation+" tags",
	)
}

func TagConflict(oldName, newName string) *Problem {
	return New(
		http.StatusConflict,
		TypeTagConflict,
		"Tag conflict",
		fmt.Sprintf("You cannot rename tag %s to %s, because it already exists", oldName, newName),
	)
}

func InvalidShortURLDeletion(shortCode, domain string) *Problem {
	p := New(
		http.StatusBadRequest,
		TypeInvalidShortURLDeletion,
		"Invalid short URL deletion",
		"Short URL could not be deleted",
	)
	p.ShortCode = shortCode
	if domain != "" {
		p.Domain = domain
	}
	return p
}

func MercureNotConfigured() *Problem {
	return New(
		http.StatusNotImplemented,
		TypeMercureNotConfigured,
		"Mercure integration not configured",
		"This Shlink instance is not integrated with a mercure hub.",
	)
}

func Write(c *gin.Context, p *Problem) {
	body, err := json.Marshal(p)
	if err != nil {
		body = []byte(`{"type":"https://shlink.io/api/error/internal","title":"Internal error","detail":"Internal error","status":500}`)
		p.Status = http.StatusInternalServerError
	}
	c.Data(p.Status, "application/problem+json; charset=utf-8", body)
}

func Abort(c *gin.Context, p *Problem) {
	Write(c, p)
	c.Abort()
}

func Encode(p *Problem) []byte {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(p)
	return bytes.TrimSpace(buf.Bytes())
}
