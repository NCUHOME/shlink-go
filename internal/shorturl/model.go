package shorturl

import (
	"time"

	"github.com/hakimyu/shlink-go/internal/api/types"
	"github.com/hakimyu/shlink-go/internal/page"
)

type VisitsSummary struct {
	Total   int64 `json:"total"`
	NonBots int64 `json:"nonBots"`
	Bots    int64 `json:"bots"`
}

type Meta struct {
	ValidSince *types.Time `json:"validSince"`
	ValidUntil *types.Time `json:"validUntil"`
	MaxVisits  *int        `json:"maxVisits"`
}

type ShortURL struct {
	ShortCode        string        `json:"shortCode"`
	ShortURL         string        `json:"shortUrl"`
	LongURL          string        `json:"longUrl"`
	DateCreated      types.Time    `json:"dateCreated"`
	VisitsSummary    VisitsSummary `json:"visitsSummary"`
	Tags             []string      `json:"tags"`
	Meta             Meta          `json:"meta"`
	Domain           *string       `json:"domain"`
	Title            *string       `json:"title"`
	Crawlable        bool          `json:"crawlable"`
	ForwardQuery     bool          `json:"forwardQuery"`
	HasRedirectRules bool          `json:"hasRedirectRules"`
}

type Pagination = page.Pagination

type ListResult struct {
	Data       []ShortURL `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Resolved struct {
	ID               int64
	DomainID         int64
	ShortCode        string
	LongURL          string
	ForwardQuery     bool
	Crawlable        bool
	DomainAuthority  string
	IsDefaultDomain  bool
	ValidSince       *time.Time
	ValidUntil       *time.Time
	MaxVisits        *int
	VisitsTotal      int64
	HasRedirectRules bool
}

type CreateParams struct {
	LongURL         string
	CustomSlug      *string
	PathPrefix      *string
	FindIfExists    bool
	Domain          *string
	ShortCodeLength *int
	ValidSince      *time.Time
	ValidUntil      *time.Time
	MaxVisits       *int
	Tags            []string
	Title           *string
	Crawlable       *bool
	ForwardQuery    *bool
}

type UpdateParams struct {
	LongURL      *string
	ValidSince   types.OmittableNullable[types.Time]
	ValidUntil   types.OmittableNullable[types.Time]
	MaxVisits    types.OmittableNullable[int]
	Tags         *[]string
	Title        types.OmittableNullable[string]
	Crawlable    *bool
	ForwardQuery *bool
}

type ListParams struct {
	Page                    int
	ItemsPerPage            int
	SearchTerm              string
	Tags                    []string
	TagsMode                string
	ExcludeTags             []string
	ExcludeTagsMode         string
	OrderBy                 string
	StartDate               *time.Time
	EndDate                 *time.Time
	ExcludeMaxVisitsReached bool
	ExcludePastValidUntil   bool
	Domain                  *string
	APIKeyName              string
}

type storedShortURL struct {
	ID               int64      `bun:"id"`
	DomainID         int64      `bun:"domain_id"`
	ShortCode        string     `bun:"short_code"`
	LongURL          string     `bun:"long_url"`
	Title            *string    `bun:"title"`
	DateCreated      time.Time  `bun:"date_created"`
	ValidSince       *time.Time `bun:"valid_since"`
	ValidUntil       *time.Time `bun:"valid_until"`
	MaxVisits        *int       `bun:"max_visits"`
	Crawlable        bool       `bun:"crawlable"`
	ForwardQuery     bool       `bun:"forward_query"`
	AuthorAPIKeyID   *int64     `bun:"author_api_key_id"`
	HasRedirectRules bool       `bun:"has_redirect_rules"`
	VisitsTotal      int64      `bun:"visits_total"`
	VisitsNonBots    int64      `bun:"visits_non_bots"`
	VisitsBots       int64      `bun:"visits_bots"`
	Authority        string     `bun:"authority"`
	IsDefault        bool       `bun:"is_default"`
}
