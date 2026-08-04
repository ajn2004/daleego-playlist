-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS show_discovery_state (
    id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    discovery_cursor TEXT NOT NULL DEFAULT '',
    last_poll_attempt_at TIMESTAMPTZ,
    last_poll_success_at TIMESTAMPTZ,
    last_poll_error TEXT
);

INSERT INTO show_discovery_state (id) VALUES (true) ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS show_discovery_state;
-- +goose StatementEnd
