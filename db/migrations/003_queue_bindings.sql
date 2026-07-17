-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS queue_playlist_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    media_server_id UUID NOT NULL REFERENCES media_servers(id),
    server_playlist_id TEXT,
    playlist_name TEXT NOT NULL DEFAULT 'TV Rotation',
    synchronized_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (playlist_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS queue_playlist_bindings;
-- +goose StatementEnd