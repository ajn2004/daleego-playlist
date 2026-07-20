package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShowProfile struct {
	ID          string    `json:"id"`
	SeriesID    string    `json:"series_id"`
	Name        string    `json:"name"`
	DefaultMode string    `json:"default_mode"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ShowProfileSeasonRule struct {
	ProfileID    string `json:"profile_id"`
	SeasonNumber int    `json:"season_number"`
	Allowed      bool   `json:"allowed"`
}

type ShowProfileEpisodeRule struct {
	ProfileID string `json:"profile_id"`
	EpisodeID string `json:"episode_id"`
	Allowed   bool   `json:"allowed"`
}

type ShowProfileRepo struct {
	pool *pgxpool.Pool
}

func NewShowProfileRepo(pool *pgxpool.Pool) *ShowProfileRepo {
	return &ShowProfileRepo{pool: pool}
}

func (r *ShowProfileRepo) Create(ctx context.Context, p *ShowProfile) (*ShowProfile, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO show_profiles (id, series_id, name, default_mode, is_default, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now(), now())
		 RETURNING id, series_id, name, default_mode, is_default, created_at, updated_at`,
		p.ID, p.SeriesID, p.Name, p.DefaultMode, p.IsDefault)
	return scanShowProfile(row)
}

func (r *ShowProfileRepo) GetByID(ctx context.Context, id string) (*ShowProfile, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, series_id, name, default_mode, is_default, created_at, updated_at
		 FROM show_profiles WHERE id = $1`, id)
	p, err := scanShowProfile(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("show profile not found")
		}
		return nil, err
	}
	return p, nil
}

func (r *ShowProfileRepo) ListBySeries(ctx context.Context, seriesID string) ([]ShowProfile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, series_id, name, default_mode, is_default, created_at, updated_at
		 FROM show_profiles WHERE series_id = $1 ORDER BY is_default DESC, created_at`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("list show profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]ShowProfile, 0)
	for rows.Next() {
		var p ShowProfile
		if err := rows.Scan(&p.ID, &p.SeriesID, &p.Name, &p.DefaultMode, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan show profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (r *ShowProfileRepo) GetDefaultForSeries(ctx context.Context, seriesID string) (*ShowProfile, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, series_id, name, default_mode, is_default, created_at, updated_at
		 FROM show_profiles WHERE series_id = $1 AND is_default`, seriesID)
	p, err := scanShowProfile(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get default show profile: %w", err)
	}
	return p, nil
}

// EnsureDefaultForSeries returns the series' default profile, creating an
// allow-all "Default" profile when none exists yet.
func (r *ShowProfileRepo) EnsureDefaultForSeries(ctx context.Context, seriesID, profileID string) (*ShowProfile, error) {
	if p, err := r.GetDefaultForSeries(ctx, seriesID); err != nil {
		return nil, err
	} else if p != nil {
		return p, nil
	}
	p, err := r.Create(ctx, &ShowProfile{
		ID:          profileID,
		SeriesID:    seriesID,
		Name:        "Default",
		DefaultMode: "allow",
		IsDefault:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("create default show profile: %w", err)
	}
	return p, nil
}

func (r *ShowProfileRepo) Update(ctx context.Context, p *ShowProfile) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE show_profiles SET name = $2, default_mode = $3, updated_at = now()
		 WHERE id = $1`, p.ID, p.Name, p.DefaultMode)
	if err != nil {
		return fmt.Errorf("update show profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("show profile not found")
	}
	return nil
}

func (r *ShowProfileRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM show_profiles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete show profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("show profile not found")
	}
	return nil
}

// SetDefault marks profileID as the series default and clears the flag on all
// other profiles of the same series.
func (r *ShowProfileRepo) SetDefault(ctx context.Context, seriesID, profileID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE show_profiles SET is_default = false, updated_at = now() WHERE series_id = $1`, seriesID); err != nil {
		return fmt.Errorf("clear default: %w", err)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE show_profiles SET is_default = true, updated_at = now() WHERE id = $1 AND series_id = $2`, profileID, seriesID)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("show profile not found")
	}
	return tx.Commit(ctx)
}

// ReplaceRules atomically replaces all season and episode rules of a profile.
func (r *ShowProfileRepo) ReplaceRules(ctx context.Context, profileID string, seasonRules []ShowProfileSeasonRule, episodeRules []ShowProfileEpisodeRule) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM show_profile_season_rules WHERE profile_id = $1`, profileID); err != nil {
		return fmt.Errorf("delete season rules: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM show_profile_episode_rules WHERE profile_id = $1`, profileID); err != nil {
		return fmt.Errorf("delete episode rules: %w", err)
	}

	for _, rule := range seasonRules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO show_profile_season_rules (id, profile_id, season_number, allowed, created_at)
			 VALUES (gen_random_uuid(), $1, $2, $3, now())`,
			profileID, rule.SeasonNumber, rule.Allowed); err != nil {
			return fmt.Errorf("insert season rule: %w", err)
		}
	}
	for _, rule := range episodeRules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO show_profile_episode_rules (id, profile_id, episode_id, allowed, created_at)
			 VALUES (gen_random_uuid(), $1, $2, $3, now())`,
			profileID, rule.EpisodeID, rule.Allowed); err != nil {
			return fmt.Errorf("insert episode rule: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE show_profiles SET updated_at = now() WHERE id = $1`, profileID); err != nil {
		return fmt.Errorf("touch profile: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *ShowProfileRepo) ListSeasonRules(ctx context.Context, profileID string) ([]ShowProfileSeasonRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT profile_id, season_number, allowed FROM show_profile_season_rules
		 WHERE profile_id = $1 ORDER BY season_number`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list season rules: %w", err)
	}
	defer rows.Close()

	rules := make([]ShowProfileSeasonRule, 0)
	for rows.Next() {
		var rule ShowProfileSeasonRule
		if err := rows.Scan(&rule.ProfileID, &rule.SeasonNumber, &rule.Allowed); err != nil {
			return nil, fmt.Errorf("scan season rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *ShowProfileRepo) ListEpisodeRules(ctx context.Context, profileID string) ([]ShowProfileEpisodeRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT profile_id, episode_id, allowed FROM show_profile_episode_rules
		 WHERE profile_id = $1 ORDER BY created_at`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list episode rules: %w", err)
	}
	defer rows.Close()

	rules := make([]ShowProfileEpisodeRule, 0)
	for rows.Next() {
		var rule ShowProfileEpisodeRule
		if err := rows.Scan(&rule.ProfileID, &rule.EpisodeID, &rule.Allowed); err != nil {
			return nil, fmt.Errorf("scan episode rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// CountAssignments returns how many playlist memberships use this profile.
func (r *ShowProfileRepo) CountAssignments(ctx context.Context, profileID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM playlist_series WHERE show_profile_id = $1`, profileID).Scan(&count)
	return count, err
}

// ReassignMemberships points every membership using fromProfileID at
// toProfileID and returns the affected memberships.
func (r *ShowProfileRepo) ReassignMemberships(ctx context.Context, fromProfileID, toProfileID string) ([]PlaylistSeries, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE playlist_series SET show_profile_id = $2
		 WHERE show_profile_id = $1
		 RETURNING id, playlist_id, series_id, mode, show_profile_id, created_at`, fromProfileID, toProfileID)
	if err != nil {
		return nil, fmt.Errorf("reassign memberships: %w", err)
	}
	defer rows.Close()

	members := make([]PlaylistSeries, 0)
	for rows.Next() {
		var ps PlaylistSeries
		if err := rows.Scan(&ps.ID, &ps.PlaylistID, &ps.SeriesID, &ps.Mode, &ps.ShowProfileID, &ps.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan reassigned member: %w", err)
		}
		members = append(members, ps)
	}
	return members, rows.Err()
}

func scanShowProfile(row pgx.Row) (*ShowProfile, error) {
	var p ShowProfile
	if err := row.Scan(&p.ID, &p.SeriesID, &p.Name, &p.DefaultMode, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}
