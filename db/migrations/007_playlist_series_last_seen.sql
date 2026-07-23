-- +goose Up
-- +goose StatementBegin

ALTER TABLE playlist_series
    ADD COLUMN last_seen_at TIMESTAMPTZ;

ALTER TABLE playlist_slots
    DROP CONSTRAINT playlist_slots_slot_type_check;

ALTER TABLE playlist_slots
    ADD CONSTRAINT playlist_slots_slot_type_check
    CHECK (slot_type IN ('top_rated', 'any', 'lowest_rated', 'least_recently_seen'));

ALTER TABLE playlist_queue_items
    DROP CONSTRAINT playlist_queue_items_slot_type_check;

ALTER TABLE playlist_queue_items
    ADD CONSTRAINT playlist_queue_items_slot_type_check
    CHECK (slot_type IN ('top_rated', 'any', 'lowest_rated', 'least_recently_seen'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE playlist_slots SET slot_type = 'any' WHERE slot_type = 'least_recently_seen';
UPDATE playlist_queue_items SET slot_type = 'any' WHERE slot_type = 'least_recently_seen';

ALTER TABLE playlist_queue_items
    DROP CONSTRAINT playlist_queue_items_slot_type_check;

ALTER TABLE playlist_queue_items
    ADD CONSTRAINT playlist_queue_items_slot_type_check
    CHECK (slot_type IN ('top_rated', 'any', 'lowest_rated'));

ALTER TABLE playlist_slots
    DROP CONSTRAINT playlist_slots_slot_type_check;

ALTER TABLE playlist_slots
    ADD CONSTRAINT playlist_slots_slot_type_check
    CHECK (slot_type IN ('top_rated', 'any', 'lowest_rated'));

ALTER TABLE playlist_series
    DROP COLUMN last_seen_at;

-- +goose StatementEnd
