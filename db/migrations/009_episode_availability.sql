-- +goose Up
-- +goose StatementBegin
ALTER TABLE episodes ADD COLUMN unavailable BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE episodes DROP COLUMN unavailable;
-- +goose StatementEnd
