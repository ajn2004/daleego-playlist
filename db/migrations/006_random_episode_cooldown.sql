-- +goose Up
-- +goose StatementBegin

ALTER TABLE playlist_series
    ADD COLUMN random_episode_cooldown INTEGER NOT NULL DEFAULT 10
    CHECK (random_episode_cooldown >= 0);

ALTER TABLE playlist_series_history
    DROP CONSTRAINT playlist_series_history_playlist_series_id_episode_id_key;

CREATE INDEX idx_playlist_series_history_recent
    ON playlist_series_history (playlist_series_id, played_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM playlist_series_history h
USING playlist_series_history newer
WHERE h.playlist_series_id = newer.playlist_series_id
  AND h.episode_id = newer.episode_id
  AND (h.played_at, h.id::text) < (newer.played_at, newer.id::text);

DROP INDEX IF EXISTS idx_playlist_series_history_recent;

ALTER TABLE playlist_series_history
    ADD CONSTRAINT playlist_series_history_playlist_series_id_episode_id_key
    UNIQUE (playlist_series_id, episode_id);

ALTER TABLE playlist_series
    DROP COLUMN random_episode_cooldown;

-- +goose StatementEnd
