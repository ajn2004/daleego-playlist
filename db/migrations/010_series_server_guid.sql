-- +goose Up
-- +goose StatementBegin
ALTER TABLE series ADD COLUMN IF NOT EXISTS server_guid TEXT;
CREATE INDEX IF NOT EXISTS series_media_server_guid_idx ON series(media_server_id, server_guid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS series_media_server_guid_idx;
ALTER TABLE series DROP COLUMN IF EXISTS server_guid;
-- +goose StatementEnd
