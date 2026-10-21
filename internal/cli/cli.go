package cli

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/uptrace/bun"

	"github.com/hakimyu/shlink-go/internal/apikey"
	"github.com/hakimyu/shlink-go/internal/app"
	"github.com/hakimyu/shlink-go/internal/config"
	"github.com/hakimyu/shlink-go/internal/domain"
	"github.com/hakimyu/shlink-go/internal/shlinkmigrate"
	"github.com/hakimyu/shlink-go/internal/store"
)

func Execute() int {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "shlink-go"}
	root.AddCommand(
		serveCommand(),
		migrateCommand(),
		migrateShlinkCommand(),
		generateAPIKeyCommand(),
		listAPIKeysCommand(),
		disableAPIKeyCommand(),
		deleteAPIKeyCommand(),
		renameAPIKeyCommand(),
	)
	return root
}

func serveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, logger, err := loadConfigAndLogger()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			db, err := store.Open(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if err := store.Migrate(ctx, db); err != nil {
				return err
			}
			if err := autoMigrateShlink(ctx, cfg, db, logger); err != nil {
				return err
			}
			if err := apikey.EnsureInitial(ctx, db.DB, cfg.InitialAPIKey); err != nil {
				return err
			}
			if err := domain.EnsureDefault(ctx, db.DB, cfg.DefaultDomain); err != nil {
				return err
			}
			application, err := app.New(cfg, db, logger)
			if err != nil {
				return err
			}
			defer func() { _ = application.Close() }()

			httpServer := &http.Server{
				Addr:              cfg.ListenAddr,
				Handler:           application.Handler(),
				ReadHeaderTimeout: 5 * time.Second,
				IdleTimeout:       60 * time.Second,
			}
			errCh := make(chan error, 1)
			go func() {
				logger.Info("http_server_started", "addr", cfg.ListenAddr)
				errCh <- httpServer.ListenAndServe()
			}()
			select {
			case err := <-errCh:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := httpServer.Shutdown(shutdownCtx); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func migrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			db, err := store.Open(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			return store.Migrate(ctx, db)
		},
	}
}

func migrateShlinkCommand() *cobra.Command {
	var sourceURL string
	var batchSize int
	cmd := &cobra.Command{
		Use:   "migrate-shlink",
		Short: "Import data from an existing Shlink PostgreSQL database",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, logger, err := loadConfigAndLogger()
			if err != nil {
				return err
			}
			if strings.TrimSpace(sourceURL) == "" {
				sourceURL = cfg.ShlinkSourceDatabaseURL
			}
			if strings.TrimSpace(sourceURL) == "" {
				return errors.New("--source-url is required (or set SHLINK_SOURCE_DATABASE_URL)")
			}
			if sourceURL == cfg.DatabaseURL {
				return errors.New("source and target database URLs must point to different databases")
			}

			ctx := context.Background()
			target, err := openMigratedDB(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = target.Close() }()
			source, err := store.Open(ctx, sourceURL)
			if err != nil {
				return err
			}
			defer func() { _ = source.Close() }()

			result, err := shlinkmigrate.Run(ctx, target, source, shlinkmigrate.Options{
				DefaultDomain: cfg.DefaultDomain,
				BatchSize:     batchSize,
				Logger:        logger,
			})
			if err != nil {
				return err
			}
			printMigrationResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceURL, "source-url", "", "Source Shlink PostgreSQL connection URL")
	cmd.Flags().IntVar(&batchSize, "batch-size", 1000, "Number of visits copied per batch")
	return cmd
}

func autoMigrateShlink(ctx context.Context, cfg config.Config, target *store.DB, logger *slog.Logger) error {
	if strings.TrimSpace(cfg.ShlinkSourceDatabaseURL) == "" {
		return nil
	}
	if cfg.ShlinkSourceDatabaseURL == cfg.DatabaseURL {
		return errors.New("SHLINK_SOURCE_DATABASE_URL and DATABASE_URL must point to different databases")
	}
	empty, err := shlinkmigrate.TargetIsEmpty(ctx, target)
	if err != nil {
		return fmt.Errorf("check target database: %w", err)
	}
	if !empty {
		logger.Info("shlink_import_skipped", "reason", "target database is not empty")
		return nil
	}
	source, err := store.Open(ctx, cfg.ShlinkSourceDatabaseURL)
	if err != nil {
		return fmt.Errorf("open Shlink source database: %w", err)
	}
	defer func() { _ = source.Close() }()

	logger.Info("shlink_import_started")
	result, err := shlinkmigrate.Run(ctx, target, source, shlinkmigrate.Options{
		DefaultDomain: cfg.DefaultDomain,
		BatchSize:     1000,
		Logger:        logger,
	})
	if errors.Is(err, shlinkmigrate.ErrTargetNotEmpty) {
		logger.Info("shlink_import_skipped", "reason", "target database is not empty")
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate Shlink data: %w", err)
	}
	printMigrationResult(result)
	return nil
}

func printMigrationResult(result *shlinkmigrate.Result) {
	fmt.Printf(
		"Imported %d domains, %d API keys, %d API key roles, %d tags, %d short URLs, %d short URL tags, %d visits and %d redirect rules.\n",
		result.Domains, result.APIKeys, result.APIKeyRoles, result.Tags,
		result.ShortURLs, result.ShortURLTags, result.Visits, result.RedirectRules,
	)
}

func generateAPIKeyCommand() *cobra.Command {
	var name string
	var expiration string
	var roles []string
	var domainAuthority string
	cmd := &cobra.Command{
		Use:   "api-key:generate",
		Short: "Generate an API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" {
				return errors.New("--name is required")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			db, err := openMigratedDB(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			var expiresAt *time.Time
			if expiration != "" {
				value, err := time.Parse(time.RFC3339Nano, expiration)
				if err != nil {
					return fmt.Errorf("invalid --expiration-date: %w", err)
				}
				expiresAt = &value
			}
			raw, err := randomKey()
			if err != nil {
				return err
			}
			tx, err := db.DB.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()
			var id int64
			if err := tx.NewRaw(
				"INSERT INTO api_keys (name, key_hash, expires_at) VALUES (?, ?, ?) RETURNING id",
				name, apikey.Hash(raw), expiresAt,
			).Scan(ctx, &id); err != nil {
				return err
			}
			for _, role := range roles {
				if err := insertRole(ctx, tx, id, role, domainAuthority); err != nil {
					return err
				}
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			fmt.Println(raw)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "API key name")
	cmd.Flags().StringVar(&expiration, "expiration-date", "", "Expiration date in RFC3339 format")
	cmd.Flags().StringArrayVar(&roles, "role", nil, "Role: AUTHORED_SHORT_URLS, DOMAIN_SPECIFIC or NO_ORPHAN_VISITS")
	cmd.Flags().StringVar(&domainAuthority, "domain", "", "Domain authority for DOMAIN_SPECIFIC")
	return cmd
}

func listAPIKeysCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "api-key:list",
		Short: "List API keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			db, err := openMigratedDB(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			rows := make([]struct {
				Name      string     `bun:"name"`
				Enabled   bool       `bun:"enabled"`
				ExpiresAt *time.Time `bun:"expires_at"`
				Roles     string     `bun:"roles"`
			}, 0)
			if err := db.DB.NewRaw(
				`SELECT k.name, k.enabled, k.expires_at,
					COALESCE(string_agg(r.role, ',' ORDER BY r.role), 'ADMIN') AS roles
				 FROM api_keys k LEFT JOIN api_key_roles r ON r.api_key_id = k.id
				 GROUP BY k.id ORDER BY k.name`,
			).Scan(ctx, &rows); err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tENABLED\tEXPIRES\tROLES")
			for _, row := range rows {
				expires := "never"
				if row.ExpiresAt != nil {
					expires = row.ExpiresAt.Format(time.RFC3339)
				}
				fmt.Fprintf(w, "%s\t%t\t%s\t%s\n", row.Name, row.Enabled, expires, row.Roles)
			}
			return w.Flush()
		},
	}
}

func disableAPIKeyCommand() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "api-key:disable",
		Short: "Disable an API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateAPIKey("disable", name, "UPDATE api_keys SET enabled = FALSE WHERE name = ?")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "API key name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func deleteAPIKeyCommand() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "api-key:delete",
		Short: "Delete an API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateAPIKey("delete", name, "DELETE FROM api_keys WHERE name = ?")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "API key name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func renameAPIKeyCommand() *cobra.Command {
	var name, newName string
	cmd := &cobra.Command{
		Use:   "api-key:rename",
		Short: "Rename an API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ctx := context.Background()
			db, err := openMigratedDB(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			result, err := db.DB.NewRaw("UPDATE api_keys SET name = ? WHERE name = ?", newName, name).Exec(ctx)
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				return sql.ErrNoRows
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Current API key name")
	cmd.Flags().StringVar(&newName, "new-name", "", "New API key name")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("new-name")
	return cmd
}

func updateAPIKey(action, name, query string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := openMigratedDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	result, err := db.DB.NewRaw(query, name).Exec(ctx)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("cannot %s API key %q: not found", action, name)
	}
	return nil
}

func insertRole(ctx context.Context, tx bun.Tx, apiKeyID int64, role, domainAuthority string) error {
	role = strings.ToUpper(strings.TrimSpace(role))
	meta := map[string]any{}
	switch role {
	case "AUTHORED_SHORT_URLS", "NO_ORPHAN_VISITS":
	case "DOMAIN_SPECIFIC":
		var domainRow domain.Domain
		if err := tx.NewSelect().Table("domains").Where("authority = ?", domain.NormalizeAuthority(domainAuthority)).Scan(ctx, &domainRow); err != nil {
			return fmt.Errorf("lookup domain %q: %w", domainAuthority, err)
		}
		if domainRow.IsDefault {
			return errors.New("DOMAIN_SPECIFIC cannot target the default domain")
		}
		meta["domain_id"] = domainRow.ID
		meta["authority"] = domainRow.Authority
	default:
		return fmt.Errorf("unsupported role %q", role)
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.NewRaw(
		"INSERT INTO api_key_roles (api_key_id, role, meta) VALUES (?, ?, ?)",
		apiKeyID, role, payload,
	).Exec(ctx)
	return err
}

func loadConfigAndLogger() (config.Config, *slog.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, err
	}
	level, _ := cfg.SlogLevel()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	return cfg, logger, nil
}

func openMigratedDB(ctx context.Context, cfg config.Config) (*store.DB, error) {
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func randomKey() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
