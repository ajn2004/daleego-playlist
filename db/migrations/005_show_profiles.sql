-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS show_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    series_id UUID NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    default_mode TEXT NOT NULL DEFAULT 'allow' CHECK (default_mode IN ('allow', 'deny')),
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (series_id, name)
);

-- Exactly one default profile per series.
CREATE UNIQUE INDEX IF NOT EXISTS idx_show_profiles_one_default
    ON show_profiles (series_id) WHERE is_default;

CREATE TABLE IF NOT EXISTS show_profile_season_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES show_profiles(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    allowed BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (profile_id, season_number)
);

CREATE TABLE IF NOT EXISTS show_profile_episode_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES show_profiles(id) ON DELETE CASCADE,
    episode_id UUID NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    allowed BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (profile_id, episode_id)
);

CREATE INDEX IF NOT EXISTS idx_show_profile_episode_rules_episode
    ON show_profile_episode_rules (episode_id);

ALTER TABLE playlist_series
    ADD COLUMN IF NOT EXISTS show_profile_id UUID REFERENCES show_profiles(id);

CREATE INDEX IF NOT EXISTS idx_playlist_series_show_profile
    ON playlist_series (show_profile_id);

-- Every existing series gets an allow-all "Default" profile.
INSERT INTO show_profiles (id, series_id, name, default_mode, is_default, created_at, updated_at)
SELECT gen_random_uuid(), s.id, 'Default', 'allow', true, now(), now()
FROM series s
WHERE NOT EXISTS (
    SELECT 1 FROM show_profiles sp WHERE sp.series_id = s.id AND sp.is_default
);

-- Existing playlist memberships attach to their series' default profile.
UPDATE playlist_series ps
SET show_profile_id = sp.id
FROM show_profiles sp
WHERE ps.series_id = sp.series_id
  AND sp.is_default
  AND ps.show_profile_id IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE playlist_series DROP COLUMN IF EXISTS show_profile_id;
DROP TABLE IF EXISTS show_profile_episode_rules;
DROP TABLE IF EXISTS show_profile_season_rules;
DROP TABLE IF EXISTS show_profiles;

-- +goose StatementEnd
