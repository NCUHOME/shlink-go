package app

import (
	"log/slog"

	"github.com/NCUnix/shlink-go/internal/api"
	"github.com/NCUnix/shlink-go/internal/apikey"
	"github.com/NCUnix/shlink-go/internal/clock"
	"github.com/NCUnix/shlink-go/internal/config"
	"github.com/NCUnix/shlink-go/internal/domain"
	"github.com/NCUnix/shlink-go/internal/geoip"
	"github.com/NCUnix/shlink-go/internal/redirect"
	"github.com/NCUnix/shlink-go/internal/redirectrule"
	"github.com/NCUnix/shlink-go/internal/server"
	"github.com/NCUnix/shlink-go/internal/shorturl"
	"github.com/NCUnix/shlink-go/internal/store"
	"github.com/NCUnix/shlink-go/internal/tag"
	"github.com/NCUnix/shlink-go/internal/visit"
)

type App struct {
	*server.App
	geo *geoip.Locator
}

func New(cfg config.Config, db *store.DB, logger *slog.Logger) (*App, error) {
	return NewWithClock(cfg, db, logger, clock.Real{})
}

func NewWithClock(cfg config.Config, db *store.DB, logger *slog.Logger, clk clock.Clock) (*App, error) {
	geo, err := geoip.Open(cfg.GeoLite2CityDBPath)
	if err != nil {
		return nil, err
	}
	auth := apikey.NewAuthenticator(db.DB)
	serverApp := server.New(cfg, db, logger, apikey.Middleware(auth))
	domains := domain.NewRepository(db.DB)
	shortURLs := shorturl.NewService(db.DB, domains, cfg, clk)
	tags := tag.NewService(db.DB)
	visits := visit.NewService(db.DB, geo, cfg, clk, logger)
	rules := redirectrule.NewService(db.DB)

	api.RegisterShortURLs(serverApp.API, shortURLs)
	api.RegisterTags(serverApp.API, tags)
	api.RegisterVisits(serverApp.API, shortURLs, tags, domains, visits)
	api.RegisterDomains(serverApp.API, domains)
	api.RegisterRedirectRules(serverApp.API, shortURLs, rules)
	api.RegisterMercure(serverApp.API)

	redirect.NewService(shortURLs, rules, visits, domains, cfg, clk, logger).Register(serverApp.Engine)
	return &App{App: serverApp, geo: geo}, nil
}

func (a *App) Close() error {
	if a == nil || a.geo == nil {
		return nil
	}
	return a.geo.Close()
}
