-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS playlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_server_id UUID NOT NULL REFERENCES media_servers(id),
    name TEXT NOT NULL,
    plex_playlist_name TEXT NOT NULL DEFAULT 'TV Rotation',
    queue_target_count INTEGER NOT NULL DEFAULT 10 CHECK (queue_target_count > 0),
    cycle_cursor INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS playlist_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    slot_type TEXT NOT NULL CHECK (slot_type IN ('top_rated', 'any', 'lowest_rated')),
    UNIQUE (playlist_id, position)
);

CREATE TABLE IF NOT EXISTS playlist_series (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    series_id UUID NOT NULL REFERENCES series(id),
    mode TEXT NOT NULL CHECK (mode IN ('serial', 'non_serial')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (playlist_id, series_id)
);

CREATE TABLE IF NOT EXISTS playlist_series_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_series_id UUID NOT NULL REFERENCES playlist_series(id) ON DELETE CASCADE,
    next_episode_id UUID REFERENCES episodes(id),
    next_position INTEGER,
    last_watched_episode_id UUID REFERENCES episodes(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (playlist_series_id)
);

CREATE TABLE IF NOT EXISTS playlist_series_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_series_id UUID NOT NULL REFERENCES playlist_series(id) ON DELETE CASCADE,
    episode_id UUID NOT NULL REFERENCES episodes(id),
    played_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (playlist_series_id, episode_id)
);

CREATE TABLE IF NOT EXISTS playlist_queue_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    cycle_index INTEGER NOT NULL,
    slot_position INTEGER NOT NULL,
    slot_type TEXT NOT NULL CHECK (slot_type IN ('top_rated', 'any', 'lowest_rated')),
    series_id UUID NOT NULL REFERENCES series(id),
    playlist_series_id UUID REFERENCES playlist_series(id),
    episode_id UUID NOT NULL REFERENCES episodes(id),
    position INTEGER NOT NULL,
    score NUMERIC,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'pushed', 'watched', 'skipped')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (playlist_id, position)
);

CREATE INDEX IF NOT EXISTS idx_playlist_queue_items_pending
    ON playlist_queue_items (playlist_id, status)
    WHERE status IN ('pending', 'pushed');

CREATE INDEX IF NOT EXISTS idx_playlist_series_history_lookup
    ON playlist_series_history (playlist_series_id, episode_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS playlist_queue_items;
DROP TABLE IF EXISTS playlist_series_history;
DROP TABLE IF EXISTS playlist_series_progress;
DROP TABLE IF EXISTS playlist_series;
DROP TABLE IF EXISTS playlist_slots;
DROP TABLE IF EXISTS playlists;
-- +goose StatementEnd