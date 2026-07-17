-- +goose Up
-- +goose StatementBegin

ALTER TABLE playlist_queue_items
    DROP CONSTRAINT playlist_queue_items_status_check;

ALTER TABLE playlist_queue_items
    ADD CONSTRAINT playlist_queue_items_status_check
    CHECK (status IN ('pending', 'pushed', 'watching', 'watched', 'skipped'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE playlist_queue_items SET status = 'pushed' WHERE status = 'watching';

ALTER TABLE playlist_queue_items
    DROP CONSTRAINT playlist_queue_items_status_check;

ALTER TABLE playlist_queue_items
    ADD CONSTRAINT playlist_queue_items_status_check
    CHECK (status IN ('pending', 'pushed', 'watched', 'skipped'));

-- +goose StatementEnd
