-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS media_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    access_token_encrypted TEXT NOT NULL,
    server_identifier TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS series (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_server_id UUID NOT NULL REFERENCES media_servers(id),
    server_series_id TEXT NOT NULL,
    library_id TEXT NOT NULL,
    title TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (media_server_id, server_series_id)
);

CREATE TABLE IF NOT EXISTS episodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    series_id UUID NOT NULL REFERENCES series(id),
    server_episode_id TEXT NOT NULL,
    season_number INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    absolute_order INTEGER NOT NULL,
    title TEXT NOT NULL,
    duration_seconds INTEGER,
    rating NUMERIC(4, 2),
    vote_count INTEGER,
    originally_available_at DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (series_id, server_episode_id),
    UNIQUE (series_id, absolute_order)
);

CREATE TABLE IF NOT EXISTS series_progress (
    series_id UUID PRIMARY KEY REFERENCES series(id),
    last_watched_episode_id UUID REFERENCES episodes(id),
    next_episode_id UUID REFERENCES episodes(id),
    last_watched_position INTEGER,
    next_position INTEGER,
    synchronized_at TIMESTAMPTZ,
    candidate_score NUMERIC,
    candidate_score_details JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rotation_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    configuration JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rotations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES rotation_profiles(id),
    status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'completed', 'cancelled')),
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    available_minutes INTEGER,
    random_seed BIGINT
);

CREATE TABLE IF NOT EXISTS rotation_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rotation_id UUID NOT NULL REFERENCES rotations(id),
    position INTEGER NOT NULL,
    series_id UUID NOT NULL REFERENCES series(id),
    episode_id UUID NOT NULL REFERENCES episodes(id),
    slot_kind TEXT NOT NULL,
    candidate_score NUMERIC NOT NULL,
    score_details JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'watched', 'skipped')),
    UNIQUE (rotation_id, position)
);

CREATE TABLE IF NOT EXISTS playlist_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_server_id UUID NOT NULL REFERENCES media_servers(id),
    rotation_profile_id UUID NOT NULL REFERENCES rotation_profiles(id),
    server_playlist_id TEXT,
    playlist_name TEXT NOT NULL DEFAULT 'TV Rotation',
    synchronized_at TIMESTAMPTZ,
    UNIQUE (media_server_id, rotation_profile_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS playlist_bindings;
DROP TABLE IF EXISTS rotation_items;
DROP TABLE IF EXISTS rotations;
DROP TABLE IF EXISTS rotation_profiles;
DROP TABLE IF EXISTS series_progress;
DROP TABLE IF EXISTS episodes;
DROP TABLE IF EXISTS series;
DROP TABLE IF EXISTS media_servers;
-- +goose StatementEnd