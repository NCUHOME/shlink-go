CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE domains (
    id BIGSERIAL PRIMARY KEY,
    authority TEXT NOT NULL UNIQUE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    base_url_redirect TEXT,
    regular_404_redirect TEXT,
    invalid_short_url_redirect TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX domains_default_unique ON domains (is_default) WHERE is_default;

CREATE TABLE api_keys (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    key_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_key_roles (
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (api_key_id, role)
);

CREATE TABLE short_urls (
    id BIGSERIAL PRIMARY KEY,
    domain_id BIGINT NOT NULL REFERENCES domains(id),
    short_code TEXT NOT NULL,
    long_url TEXT NOT NULL,
    title TEXT,
    date_created TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_since TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    max_visits INTEGER,
    crawlable BOOLEAN NOT NULL DEFAULT FALSE,
    forward_query BOOLEAN NOT NULL DEFAULT TRUE,
    author_api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    has_redirect_rules BOOLEAN NOT NULL DEFAULT FALSE,
    visits_total BIGINT NOT NULL DEFAULT 0,
    visits_non_bots BIGINT NOT NULL DEFAULT 0,
    visits_bots BIGINT NOT NULL DEFAULT 0,
    UNIQUE (domain_id, short_code)
);

CREATE INDEX short_urls_date_created_idx ON short_urls (date_created);
CREATE INDEX short_urls_visits_total_idx ON short_urls (visits_total);
CREATE INDEX short_urls_visits_non_bots_idx ON short_urls (visits_non_bots);
CREATE INDEX short_urls_short_code_trgm_idx ON short_urls USING GIN (short_code gin_trgm_ops);
CREATE INDEX short_urls_long_url_trgm_idx ON short_urls USING GIN (long_url gin_trgm_ops);

CREATE TABLE tags (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE short_url_tags (
    short_url_id BIGINT NOT NULL REFERENCES short_urls(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (short_url_id, tag_id)
);

CREATE INDEX short_url_tags_tag_id_short_url_id_idx ON short_url_tags (tag_id, short_url_id);

CREATE TABLE visits (
    id BIGSERIAL PRIMARY KEY,
    short_url_id BIGINT REFERENCES short_urls(id) ON DELETE CASCADE,
    domain_id BIGINT REFERENCES domains(id) ON DELETE SET NULL,
    visited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    referer TEXT,
    user_agent TEXT,
    remote_ip TEXT,
    visited_url TEXT,
    redirect_url TEXT,
    potential_bot BOOLEAN NOT NULL DEFAULT FALSE,
    orphan_type TEXT CHECK (orphan_type IS NULL OR orphan_type IN ('base_url', 'invalid_short_url', 'regular_404')),
    country_code TEXT,
    country_name TEXT,
    region_name TEXT,
    city_name TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    timezone TEXT
);

CREATE INDEX visits_short_url_id_visited_at_idx ON visits (short_url_id, visited_at DESC);
CREATE INDEX visits_visited_at_idx ON visits (visited_at DESC);
CREATE INDEX visits_domain_id_visited_at_idx ON visits (domain_id, visited_at DESC);
CREATE INDEX visits_orphan_visited_at_idx ON visits (visited_at DESC) WHERE orphan_type IS NOT NULL;

CREATE TABLE redirect_rules (
    id BIGSERIAL PRIMARY KEY,
    short_url_id BIGINT NOT NULL REFERENCES short_urls(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL,
    long_url TEXT NOT NULL,
    conditions JSONB NOT NULL DEFAULT '[]'::jsonb,
    UNIQUE (short_url_id, priority)
);
