-- +goose Up
-- +goose StatementBegin

DO $$
DECLARE
    constraint_name text;
BEGIN
    FOR constraint_name IN
        SELECT con.conname
        FROM pg_constraint con
        JOIN pg_class rel ON rel.oid = con.conrelid
        JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
        WHERE rel.relname = 'episodes'
          AND nsp.nspname = current_schema()
          AND con.contype = 'u'
          AND pg_get_constraintdef(con.oid) = 'UNIQUE (series_id, absolute_order)'
    LOOP
        EXECUTE format('ALTER TABLE episodes DROP CONSTRAINT %I', constraint_name);
    END LOOP;
END $$;

CREATE UNIQUE INDEX episodes_available_absolute_order_idx
    ON episodes (series_id, absolute_order)
    WHERE unavailable = false;

UPDATE series_progress p
SET next_position = e.absolute_order
FROM episodes e
WHERE e.id = p.next_episode_id
  AND e.series_id = p.series_id
  AND e.unavailable = false;

UPDATE playlist_series_progress p
SET next_position = e.absolute_order
FROM playlist_series ps, episodes e
WHERE ps.id = p.playlist_series_id
  AND e.id = p.next_episode_id
  AND e.series_id = ps.series_id
  AND e.unavailable = false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS episodes_available_absolute_order_idx;
ALTER TABLE episodes ADD CONSTRAINT episodes_series_id_absolute_order_key UNIQUE (series_id, absolute_order);
-- +goose StatementEnd
