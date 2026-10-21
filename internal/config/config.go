package config

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/caarlos0/env/v11"
)

const (
	DefaultShortCodesLength = 5
	DefaultShortCodesChars  = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type Config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`
	ListenAddr  string `env:"LISTEN_ADDR" envDefault:":8080"`

	ShlinkSourceDatabaseURL string `env:"SHLINK_SOURCE_DATABASE_URL"`

	DefaultDomain  string `env:"DEFAULT_DOMAIN,required"`
	ISHTTPSEnabled bool   `env:"IS_HTTPS_ENABLED" envDefault:"false"`
	InitialAPIKey  string `env:"INITIAL_API_KEY"`

	GeoLite2CityDBPath   string   `env:"GEOLITE2_CITY_DB_PATH"`
	TrustedProxies       []string `env:"TRUSTED_PROXIES" envSeparator:"," envDefault:"127.0.0.1,::1"`
	CORSAllowedOrigins   []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"*"`
	CORSAllowCredentials bool     `env:"CORS_ALLOW_CREDENTIALS" envDefault:"false"`
	CORSMaxAge           int      `env:"CORS_MAX_AGE" envDefault:"3600"`

	ShortURLTrailingSlash bool   `env:"SHORT_URL_TRAILING_SLASH" envDefault:"false"`
	ExtraPathMode         string `env:"EXTRA_PATH_MODE" envDefault:"default"`
	DisableTrackParam     string `env:"DISABLE_TRACK_PARAM"`

	DefaultShortCodesLength int    `env:"DEFAULT_SHORT_CODES_LENGTH" envDefault:"5"`
	DefaultShortCodesChars  string `env:"DEFAULT_SHORT_CODES_CHARS" envDefault:"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"`

	AnonymizeRemoteIP bool `env:"ANONYMIZE_REMOTE_IP" envDefault:"true"`
	TrackOrphanVisits bool `env:"TRACK_ORPHAN_VISITS" envDefault:"true"`

	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if strings.TrimSpace(c.DefaultDomain) == "" {
		return fmt.Errorf("DEFAULT_DOMAIN is required")
	}
	if strings.ContainsAny(c.DefaultDomain, "/\\ ") {
		return fmt.Errorf("DEFAULT_DOMAIN must be a host name without scheme or path")
	}
	if _, _, err := net.SplitHostPort(c.DefaultDomain); err != nil {
		// No port is the common case; a bare host or host:port is both valid.
		if strings.Count(c.DefaultDomain, ":") > 1 {
			return fmt.Errorf("DEFAULT_DOMAIN is not a valid host")
		}
	}
	switch c.ExtraPathMode {
	case "default", "append", "ignore":
	default:
		return fmt.Errorf("EXTRA_PATH_MODE must be one of default, append, ignore")
	}
	if c.DefaultShortCodesLength <= 0 {
		return fmt.Errorf("DEFAULT_SHORT_CODES_LENGTH must be greater than zero")
	}
	if c.DefaultShortCodesChars == "" {
		return fmt.Errorf("DEFAULT_SHORT_CODES_CHARS must not be empty")
	}
	if _, err := c.SlogLevel(); err != nil {
		return err
	}
	if c.CORSMaxAge < 0 {
		return fmt.Errorf("CORS_MAX_AGE must not be negative")
	}
	return nil
}

func (c Config) SlogLevel() (slog.Level, error) {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}
}

func (c Config) DisabledTrackParams() []string {
	if strings.TrimSpace(c.DisableTrackParam) == "" {
		return nil
	}
	parts := strings.Split(c.DisableTrackParam, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
