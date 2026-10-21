package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/shlink")
	t.Setenv("DEFAULT_DOMAIN", "s.example.com")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.ListenAddr)
	require.Equal(t, DefaultShortCodesLength, cfg.DefaultShortCodesLength)
	require.Equal(t, DefaultShortCodesChars, cfg.DefaultShortCodesChars)
	require.Equal(t, "default", cfg.ExtraPathMode)
	require.Equal(t, []string{"127.0.0.1", "::1"}, cfg.TrustedProxies)
	require.Equal(t, []string{"*"}, cfg.CORSAllowedOrigins)
	require.True(t, cfg.AnonymizeRemoteIP)
	require.True(t, cfg.TrackOrphanVisits)
	require.Equal(t, slog.LevelInfo, mustLevel(t, cfg))
}

func TestValidateRejectsInvalidExtraPathMode(t *testing.T) {
	cfg := Config{
		DatabaseURL:             "postgres://localhost/shlink",
		DefaultDomain:           "s.example.com",
		ListenAddr:              ":8080",
		ExtraPathMode:           "drop",
		DefaultShortCodesLength: 5,
		DefaultShortCodesChars:  "abc",
		LogLevel:                "info",
	}
	require.ErrorContains(t, cfg.Validate(), "EXTRA_PATH_MODE")
}

func TestDisabledTrackParams(t *testing.T) {
	cfg := Config{DisableTrackParam: "utm_source, utm_medium,,utm_campaign"}
	require.Equal(t, []string{"utm_source", "utm_medium", "utm_campaign"}, cfg.DisabledTrackParams())
}

func mustLevel(t *testing.T, cfg Config) slog.Level {
	t.Helper()
	level, err := cfg.SlogLevel()
	require.NoError(t, err)
	return level
}
