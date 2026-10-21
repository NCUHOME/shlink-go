package shlinkmigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hakimyu/shlink-go/internal/store"
)

var ErrTargetNotEmpty = errors.New("target database is not empty")

type Options struct {
	DefaultDomain string
	BatchSize     int
	Logger        *slog.Logger
}

type Result struct {
	Domains       int64
	APIKeys       int64
	APIKeyRoles   int64
	Tags          int64
	ShortURLs     int64
	ShortURLTags  int64
	Visits        int64
	RedirectRules int64
}

type sourceDomain struct {
	ID                      int64
	Authority               string
	IsDefault               bool
	BaseURLRedirect         *string
	Regular404Redirect      *string
	InvalidShortURLRedirect *string
}

func Run(ctx context.Context, target, source *store.DB, opts Options) (*Result, error) {
	if strings.TrimSpace(opts.DefaultDomain) == "" {
		return nil, errors.New("default domain is required")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if target == nil || source == nil {
		return nil, errors.New("target and source databases are required")
	}
	if err := validateSourceSchema(ctx, source); err != nil {
		return nil, err
	}

	tx, err := target.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin target transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(783412119)"); err != nil {
		return nil, fmt.Errorf("lock target database: %w", err)
	}
	if err := ensureTargetEmpty(ctx, tx); err != nil {
		return nil, err
	}

	result := new(Result)
	result.Domains, err = copyDomains(ctx, tx, source, opts.DefaultDomain)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "domains", "rows", result.Domains)

	result.APIKeys, result.APIKeyRoles, err = copyAPIKeys(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "api_keys", "rows", result.APIKeys, "roles", result.APIKeyRoles)

	result.Tags, err = copyTags(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "tags", "rows", result.Tags)

	result.ShortURLs, err = copyShortURLs(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "short_urls", "rows", result.ShortURLs)

	result.ShortURLTags, err = copyShortURLTags(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "short_urls_in_tags", "rows", result.ShortURLTags)

	result.Visits, err = copyVisits(ctx, tx, source, opts.BatchSize, opts.Logger)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "visits", "rows", result.Visits)

	result.RedirectRules, err = copyRedirectRules(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("shlink_import_progress", "table", "short_url_redirect_rules", "rows", result.RedirectRules)

	if err := resetSequences(ctx, tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit shlink import: %w", err)
	}
	return result, nil
}

func validateSourceSchema(ctx context.Context, source *store.DB) error {
	required := map[string][]string{
		"domains":                  {"id", "authority", "base_url_redirect", "regular_not_found_redirect", "invalid_short_url_redirect"},
		"api_keys":                 {"id", "key", "name", "expiration_date", "enabled"},
		"api_key_roles":            {"id", "role_name", "meta", "api_key_id"},
		"tags":                     {"id", "name"},
		"short_urls":               {"id", "domain_id", "short_code", "original_url", "title", "date_created", "valid_since", "valid_until", "max_visits", "crawlable", "forward_query", "author_api_key_id"},
		"short_urls_in_tags":       {"short_url_id", "tag_id"},
		"visits":                   {"id", "short_url_id", "date", "referer", "user_agent", "remote_addr", "visited_url", "redirect_url", "potential_bot", "type", "visit_location_id"},
		"visit_locations":          {"id", "country_code", "country_name", "region_name", "city_name", "timezone", "lat", "lon", "is_empty"},
		"short_url_visits_counts":  {"short_url_id", "potential_bot", "count"},
		"short_url_redirect_rules": {"id", "short_url_id", "priority", "long_url"},
		"redirect_conditions":      {"id", "type", "match_key", "match_value"},
		"redirect_conditions_in_short_url_redirect_rules": {"short_url_redirect_rule_id", "redirect_condition_id"},
	}

	rows, err := source.SQL.QueryContext(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema()`)
	if err != nil {
		return fmt.Errorf("inspect source schema: %w", err)
	}
	defer rows.Close()

	found := make(map[string]map[string]bool)
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return fmt.Errorf("scan source schema: %w", err)
		}
		if found[table] == nil {
			found[table] = make(map[string]bool)
		}
		found[table][column] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read source schema: %w", err)
	}

	for table, columns := range required {
		for _, column := range columns {
			if !found[table][column] {
				return fmt.Errorf("unsupported Shlink schema: missing column %s.%s", table, column)
			}
		}
	}
	return nil
}

func ensureTargetEmpty(ctx context.Context, tx *sql.Tx) error {
	var occupied bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM domains)
		    OR EXISTS (SELECT 1 FROM api_keys)
		    OR EXISTS (SELECT 1 FROM tags)
		    OR EXISTS (SELECT 1 FROM short_urls)
		    OR EXISTS (SELECT 1 FROM visits)
		    OR EXISTS (SELECT 1 FROM redirect_rules)`).Scan(&occupied)
	if err != nil {
		return fmt.Errorf("check target database: %w", err)
	}
	if occupied {
		return ErrTargetNotEmpty
	}
	return nil
}

func copyDomains(ctx context.Context, tx *sql.Tx, source *store.DB, defaultDomain string) (int64, error) {
	rows, err := source.SQL.QueryContext(ctx, `
		SELECT id, authority, base_url_redirect, regular_not_found_redirect, invalid_short_url_redirect
		FROM domains ORDER BY id`)
	if err != nil {
		return 0, fmt.Errorf("read source domains: %w", err)
	}
	defer rows.Close()

	defaultDomain = normalizeAuthority(defaultDomain)
	domains := make([]sourceDomain, 0)
	var maxID int64
	foundDefault := false
	for rows.Next() {
		var item sourceDomain
		var base, regular, invalid sql.NullString
		if err := rows.Scan(&item.ID, &item.Authority, &base, &regular, &invalid); err != nil {
			return 0, fmt.Errorf("scan source domain: %w", err)
		}
		item.Authority = normalizeAuthority(item.Authority)
		item.BaseURLRedirect = nullString(base)
		item.Regular404Redirect = nullString(regular)
		item.InvalidShortURLRedirect = nullString(invalid)
		item.IsDefault = item.Authority == defaultDomain
		foundDefault = foundDefault || item.IsDefault
		if item.ID > maxID {
			maxID = item.ID
		}
		domains = append(domains, item)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read source domains: %w", err)
	}
	if !foundDefault {
		maxID++
		domains = append(domains, sourceDomain{
			ID:        maxID,
			Authority: defaultDomain,
			IsDefault: true,
		})
	}

	for _, item := range domains {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO domains
				(id, authority, is_default, base_url_redirect, regular_404_redirect, invalid_short_url_redirect)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			item.ID, item.Authority, item.IsDefault, item.BaseURLRedirect,
			item.Regular404Redirect, item.InvalidShortURLRedirect,
		); err != nil {
			return 0, fmt.Errorf("insert domain %d: %w", item.ID, err)
		}
	}
	return int64(len(domains)), nil
}

func copyAPIKeys(ctx context.Context, tx *sql.Tx, source *store.DB) (int64, int64, error) {
	rows, err := source.SQL.QueryContext(ctx, `
		SELECT id, key, name, expiration_date, enabled
		FROM api_keys ORDER BY id`)
	if err != nil {
		return 0, 0, fmt.Errorf("read source API keys: %w", err)
	}
	defer rows.Close()

	usedNames := make(map[string]bool)
	var keyCount int64
	for rows.Next() {
		var id int64
		var key string
		var name sql.NullString
		var expiration sql.NullTime
		var enabled bool
		if err := rows.Scan(&id, &key, &name, &expiration, &enabled); err != nil {
			return 0, 0, fmt.Errorf("scan source API key: %w", err)
		}
		keyName := strings.TrimSpace(name.String)
		if keyName == "" {
			keyName = fmt.Sprintf("api-key-%d", id)
		}
		baseName := keyName
		suffix := 1
		for usedNames[keyName] {
			keyName = fmt.Sprintf("%s-%d", baseName, suffix)
			suffix++
		}
		usedNames[keyName] = true

		var expiresAt *time.Time
		if expiration.Valid {
			value := expiration.Time
			expiresAt = &value
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO api_keys (id, name, key_hash, expires_at, enabled)
			VALUES ($1, $2, $3, $4, $5)`,
			id, keyName, key, expiresAt, enabled,
		); err != nil {
			return 0, 0, fmt.Errorf("insert API key %d: %w", id, err)
		}
		keyCount++
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("read source API keys: %w", err)
	}

	roles, err := source.SQL.QueryContext(ctx, `
		SELECT api_key_id, role_name, COALESCE(meta, '{}'::jsonb)::text
		FROM api_key_roles ORDER BY id`)
	if err != nil {
		return 0, 0, fmt.Errorf("read source API key roles: %w", err)
	}
	defer roles.Close()
	var roleCount int64
	for roles.Next() {
		var apiKeyID int64
		var role, meta string
		if err := roles.Scan(&apiKeyID, &role, &meta); err != nil {
			return 0, 0, fmt.Errorf("scan source API key role: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO api_key_roles (api_key_id, role, meta)
			VALUES ($1, $2, $3::jsonb)`,
			apiKeyID, role, meta,
		); err != nil {
			return 0, 0, fmt.Errorf("insert API key role for %d: %w", apiKeyID, err)
		}
		roleCount++
	}
	if err := roles.Err(); err != nil {
		return 0, 0, fmt.Errorf("read source API key roles: %w", err)
	}
	return keyCount, roleCount, nil
}

func copyTags(ctx context.Context, tx *sql.Tx, source *store.DB) (int64, error) {
	rows, err := source.SQL.QueryContext(ctx, "SELECT id, name FROM tags ORDER BY id")
	if err != nil {
		return 0, fmt.Errorf("read source tags: %w", err)
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return 0, fmt.Errorf("scan source tag: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO tags (id, name) VALUES ($1, $2)", id, name); err != nil {
			return 0, fmt.Errorf("insert tag %d: %w", id, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read source tags: %w", err)
	}
	return count, nil
}

func copyShortURLs(ctx context.Context, tx *sql.Tx, source *store.DB) (int64, error) {
	rows, err := source.SQL.QueryContext(ctx, `
		SELECT s.id, s.domain_id, s.short_code, s.original_url, s.title, s.date_created,
			s.valid_since, s.valid_until, s.max_visits, s.crawlable, s.forward_query,
			s.author_api_key_id,
			EXISTS (SELECT 1 FROM short_url_redirect_rules r WHERE r.short_url_id = s.id),
			COALESCE(c.visits_total, 0), COALESCE(c.visits_non_bots, 0), COALESCE(c.visits_bots, 0)
		FROM short_urls s
		LEFT JOIN (
			SELECT short_url_id,
				SUM(count) AS visits_total,
				SUM(count) FILTER (WHERE NOT potential_bot) AS visits_non_bots,
				SUM(count) FILTER (WHERE potential_bot) AS visits_bots
			FROM short_url_visits_counts
			GROUP BY short_url_id
		) c ON c.short_url_id = s.id
		ORDER BY s.id`)
	if err != nil {
		return 0, fmt.Errorf("read source short URLs: %w", err)
	}
	defer rows.Close()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO short_urls
			(id, domain_id, short_code, long_url, title, date_created, valid_since, valid_until,
			 max_visits, crawlable, forward_query, author_api_key_id, has_redirect_rules,
			 visits_total, visits_non_bots, visits_bots)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`)
	if err != nil {
		return 0, fmt.Errorf("prepare short URL insert: %w", err)
	}
	defer stmt.Close()

	var count int64
	for rows.Next() {
		var id, domainID int64
		var shortCode, longURL string
		var title sql.NullString
		var authorID sql.NullInt64
		var dateCreated time.Time
		var validSince, validUntil sql.NullTime
		var maxVisits sql.NullInt64
		var crawlable, forwardQuery, hasRedirectRules bool
		var visitsTotal, visitsNonBots, visitsBots int64
		if err := rows.Scan(
			&id, &domainID, &shortCode, &longURL, &title, &dateCreated,
			&validSince, &validUntil, &maxVisits, &crawlable, &forwardQuery,
			&authorID, &hasRedirectRules, &visitsTotal, &visitsNonBots, &visitsBots,
		); err != nil {
			return 0, fmt.Errorf("scan source short URL: %w", err)
		}
		if _, err := stmt.ExecContext(ctx,
			id, domainID, shortCode, longURL, nullString(title), dateCreated,
			nullTime(validSince), nullTime(validUntil), nullInt(maxVisits), crawlable, forwardQuery,
			nullInt64(authorID), hasRedirectRules, visitsTotal, visitsNonBots, visitsBots,
		); err != nil {
			return 0, fmt.Errorf("insert short URL %d: %w", id, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read source short URLs: %w", err)
	}
	return count, nil
}

func copyShortURLTags(ctx context.Context, tx *sql.Tx, source *store.DB) (int64, error) {
	rows, err := source.SQL.QueryContext(ctx, `
		SELECT short_url_id, tag_id FROM short_urls_in_tags ORDER BY short_url_id, tag_id`)
	if err != nil {
		return 0, fmt.Errorf("read source short URL tags: %w", err)
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var shortURLID, tagID int64
		if err := rows.Scan(&shortURLID, &tagID); err != nil {
			return 0, fmt.Errorf("scan source short URL tag: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO short_url_tags (short_url_id, tag_id) VALUES ($1, $2)",
			shortURLID, tagID,
		); err != nil {
			return 0, fmt.Errorf("insert short URL tag: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read source short URL tags: %w", err)
	}
	return count, nil
}

func copyVisits(ctx context.Context, tx *sql.Tx, source *store.DB, batchSize int, logger *slog.Logger) (int64, error) {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO visits
			(id, short_url_id, domain_id, visited_at, referer, user_agent, remote_ip,
			 visited_url, redirect_url, potential_bot, orphan_type,
			 country_code, country_name, region_name, city_name, latitude, longitude, timezone)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`)
	if err != nil {
		return 0, fmt.Errorf("prepare visit insert: %w", err)
	}
	defer stmt.Close()

	var lastID int64
	var count int64
	for {
		rows, err := source.SQL.QueryContext(ctx, `
			SELECT v.id, v.short_url_id, s.domain_id, v.date, v.referer, v.user_agent, v.remote_addr,
				v.visited_url, v.redirect_url, v.potential_bot, v.type,
				l.country_code, l.country_name, l.region_name, l.city_name,
				l.lat, l.lon, l.timezone, l.is_empty
			FROM visits v
			LEFT JOIN short_urls s ON s.id = v.short_url_id
			LEFT JOIN visit_locations l ON l.id = v.visit_location_id
			WHERE v.id > $1
			ORDER BY v.id
			LIMIT $2`,
			lastID, batchSize,
		)
		if err != nil {
			return 0, fmt.Errorf("read source visits: %w", err)
		}

		batchCount := 0
		for rows.Next() {
			var id int64
			var shortURLID, domainID sql.NullInt64
			var visitedAt time.Time
			var referer, userAgent, remoteIP, visitedURL, redirectURL sql.NullString
			var potentialBot bool
			var visitType string
			var countryCode, countryName, regionName, cityName, timezone sql.NullString
			var latitude, longitude sql.NullFloat64
			var locationEmpty sql.NullBool
			if err := rows.Scan(
				&id, &shortURLID, &domainID, &visitedAt, &referer, &userAgent, &remoteIP,
				&visitedURL, &redirectURL, &potentialBot, &visitType,
				&countryCode, &countryName, &regionName, &cityName,
				&latitude, &longitude, &timezone, &locationEmpty,
			); err != nil {
				_ = rows.Close()
				return 0, fmt.Errorf("scan source visit: %w", err)
			}
			if locationEmpty.Valid && locationEmpty.Bool {
				countryCode, countryName, regionName, cityName, timezone = sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{}
				latitude, longitude = sql.NullFloat64{}, sql.NullFloat64{}
			}
			if _, err := stmt.ExecContext(ctx,
				id, nullInt64(shortURLID), nullInt64(domainID), visitedAt,
				nullString(referer), nullString(userAgent), nullString(remoteIP),
				nullString(visitedURL), nullString(redirectURL), potentialBot,
				orphanType(visitType), nullString(countryCode), nullString(countryName),
				nullString(regionName), nullString(cityName), nullFloat(latitude),
				nullFloat(longitude), nullString(timezone),
			); err != nil {
				_ = rows.Close()
				return 0, fmt.Errorf("insert visit %d: %w", id, err)
			}
			lastID = id
			count++
			batchCount++
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("read source visits: %w", err)
		}
		_ = rows.Close()
		if batchCount == 0 {
			break
		}
		logger.Debug("shlink_import_visits_batch", "rows", batchCount, "last_id", lastID)
	}
	return count, nil
}

func copyRedirectRules(ctx context.Context, tx *sql.Tx, source *store.DB) (int64, error) {
	rows, err := source.SQL.QueryContext(ctx, `
		SELECT r.id, r.short_url_id, r.priority, r.long_url,
			COALESCE(
				json_agg(
					json_build_object(
						'type', c.type,
						'matchKey', c.match_key,
						'matchValue', c.match_value
					) ORDER BY c.id
				) FILTER (WHERE c.id IS NOT NULL),
				'[]'::json
			)::text
		FROM short_url_redirect_rules r
		LEFT JOIN redirect_conditions_in_short_url_redirect_rules rc
			ON rc.short_url_redirect_rule_id = r.id
		LEFT JOIN redirect_conditions c ON c.id = rc.redirect_condition_id
		GROUP BY r.id, r.short_url_id, r.priority, r.long_url
		ORDER BY r.short_url_id, r.priority`)
	if err != nil {
		return 0, fmt.Errorf("read source redirect rules: %w", err)
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var id, shortURLID int64
		var priority int
		var longURL, conditions string
		if err := rows.Scan(&id, &shortURLID, &priority, &longURL, &conditions); err != nil {
			return 0, fmt.Errorf("scan source redirect rule: %w", err)
		}
		if !json.Valid([]byte(conditions)) {
			return 0, fmt.Errorf("invalid conditions JSON for redirect rule %d", id)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO redirect_rules (id, short_url_id, priority, long_url, conditions)
			VALUES ($1, $2, $3, $4, $5::jsonb)`,
			id, shortURLID, priority, longURL, conditions,
		); err != nil {
			return 0, fmt.Errorf("insert redirect rule %d: %w", id, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read source redirect rules: %w", err)
	}
	return count, nil
}

func resetSequences(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"domains", "api_keys", "tags", "short_urls", "visits", "redirect_rules"} {
		query := fmt.Sprintf(`
			SELECT setval(
				pg_get_serial_sequence('%s', 'id'),
				COALESCE((SELECT MAX(id) FROM %s), 1),
				EXISTS (SELECT 1 FROM %s)
			)`, table, table, table)
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("reset %s id sequence: %w", table, err)
		}
	}
	return nil
}

func orphanType(value string) any {
	switch value {
	case "base_url", "invalid_short_url", "regular_404":
		return value
	default:
		return nil
	}
}

func normalizeAuthority(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	number := int(value.Int64)
	return &number
}

func nullInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	number := value.Int64
	return &number
}

func nullFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	number := value.Float64
	return &number
}
