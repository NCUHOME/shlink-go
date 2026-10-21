package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/hakimyu/shlink-go/internal/apikey"
)

type Domain struct {
	ID                      int64     `bun:"id"`
	Authority               string    `bun:"authority"`
	IsDefault               bool      `bun:"is_default"`
	BaseURLRedirect         *string   `bun:"base_url_redirect"`
	Regular404Redirect      *string   `bun:"regular_404_redirect"`
	InvalidShortURLRedirect *string   `bun:"invalid_short_url_redirect"`
	CreatedAt               time.Time `bun:"created_at"`
}

type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

type RedirectsPatch struct {
	BaseURLRedirect            *string
	BaseURLRedirectSet         bool
	Regular404Redirect         *string
	Regular404RedirectSet      bool
	InvalidShortURLRedirect    *string
	InvalidShortURLRedirectSet bool
}

func EnsureDefault(ctx context.Context, db *bun.DB, authority string) error {
	authority = NormalizeAuthority(authority)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin default domain transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.NewRaw(
		"UPDATE domains SET is_default = FALSE WHERE is_default = TRUE AND authority <> ?",
		authority,
	).Exec(ctx); err != nil {
		return fmt.Errorf("clear previous default domain: %w", err)
	}
	if _, err := tx.NewRaw(
		`INSERT INTO domains (authority, is_default) VALUES (?, TRUE)
		 ON CONFLICT (authority) DO UPDATE SET is_default = TRUE`,
		authority,
	).Exec(ctx); err != nil {
		return fmt.Errorf("create default domain: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit default domain: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id int64) (*Domain, error) {
	d := new(Domain)
	if err := r.db.NewSelect().Table("domains").Where("id = ?", id).Scan(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (r *Repository) GetByAuthority(ctx context.Context, authority string) (*Domain, error) {
	d := new(Domain)
	if err := r.db.NewSelect().Table("domains").Where("authority = ?", NormalizeAuthority(authority)).Scan(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (r *Repository) GetDefault(ctx context.Context) (*Domain, error) {
	d := new(Domain)
	if err := r.db.NewSelect().Table("domains").Where("is_default = TRUE").Limit(1).Scan(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (r *Repository) ListVisible(ctx context.Context, scope apikey.Scope) ([]Domain, error) {
	where := make([]string, 0, 1)
	args := make([]any, 0, 1)
	if scope.DomainID != nil {
		where = append(where, "d.id = ?")
		args = append(args, *scope.DomainID)
	} else if scope.AuthorAPIKeyID != nil {
		where = append(where, `(d.is_default = TRUE OR EXISTS (
			SELECT 1 FROM short_urls s WHERE s.domain_id = d.id AND s.author_api_key_id = ?
		))`)
		args = append(args, *scope.AuthorAPIKeyID)
	} else {
		where = append(where, `(d.is_default = TRUE
			OR d.base_url_redirect IS NOT NULL
			OR d.regular_404_redirect IS NOT NULL
			OR d.invalid_short_url_redirect IS NOT NULL
			OR EXISTS (SELECT 1 FROM short_urls s WHERE s.domain_id = d.id))`)
	}
	if len(where) == 0 {
		where = append(where, "TRUE")
	}
	domains := make([]Domain, 0)
	if err := r.db.NewRaw(
		"SELECT d.* FROM domains d WHERE "+strings.Join(where, " AND ")+
			" ORDER BY d.is_default DESC, d.authority ASC",
		args...,
	).Scan(ctx, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

func (r *Repository) UpdateRedirects(ctx context.Context, authority string, patch RedirectsPatch) (*Domain, error) {
	authority = NormalizeAuthority(authority)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var current Domain
	err = tx.NewSelect().Table("domains").Where("authority = ?", authority).Scan(ctx, &current)
	if errors.Is(err, sql.ErrNoRows) {
		current = Domain{Authority: authority}
	} else if err != nil {
		return nil, err
	}

	if patch.BaseURLRedirectSet {
		current.BaseURLRedirect = patch.BaseURLRedirect
	}
	if patch.Regular404RedirectSet {
		current.Regular404Redirect = patch.Regular404Redirect
	}
	if patch.InvalidShortURLRedirectSet {
		current.InvalidShortURLRedirect = patch.InvalidShortURLRedirect
	}

	if current.ID == 0 {
		if _, err := tx.NewRaw(
			`INSERT INTO domains
				(authority, base_url_redirect, regular_404_redirect, invalid_short_url_redirect)
			 VALUES (?, ?, ?, ?)`,
			current.Authority, current.BaseURLRedirect, current.Regular404Redirect, current.InvalidShortURLRedirect,
		).Exec(ctx); err != nil {
			return nil, err
		}
	} else if _, err := tx.NewRaw(
		`UPDATE domains SET base_url_redirect = ?, regular_404_redirect = ?, invalid_short_url_redirect = ?
		 WHERE id = ?`,
		current.BaseURLRedirect, current.Regular404Redirect, current.InvalidShortURLRedirect, current.ID,
	).Exec(ctx); err != nil {
		return nil, err
	}

	if err := tx.NewSelect().Table("domains").Where("authority = ?", authority).Scan(ctx, &current); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &current, nil
}

func NormalizeAuthority(authority string) string {
	return strings.ToLower(strings.TrimSpace(authority))
}
