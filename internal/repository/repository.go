package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

func (q *Queries) Ping(ctx context.Context) error {
	return q.pool.Ping(ctx)
}

// --- Media Server ---

type MediaServer struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Token     string    `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ServerRepo struct {
	pool *pgxpool.Pool
}

func NewServerRepo(pool *pgxpool.Pool) *ServerRepo {
	return &ServerRepo{pool: pool}
}

func (r *ServerRepo) Create(ctx context.Context, id, url, token, name string) (*MediaServer, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO media_servers (id, base_url, access_token_encrypted, name) VALUES ($1, $2, $3, $4)`,
		id, url, token, name)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}
	return &MediaServer{ID: id, URL: url, Token: token, Name: name, CreatedAt: time.Now()}, nil
}

func (r *ServerRepo) List(ctx context.Context) ([]MediaServer, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, base_url, access_token_encrypted, name, created_at FROM media_servers ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	var servers []MediaServer
	for rows.Next() {
		var s MediaServer
		if err := rows.Scan(&s.ID, &s.URL, &s.Token, &s.Name, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (r *ServerRepo) GetByID(ctx context.Context, id string) (*MediaServer, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, base_url, access_token_encrypted, name, created_at FROM media_servers WHERE id = $1`, id)
	var s MediaServer
	if err := row.Scan(&s.ID, &s.URL, &s.Token, &s.Name, &s.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("server not found")
		}
		return nil, fmt.Errorf("get server: %w", err)
	}
	return &s, nil
}

// --- Series ---

type Series struct {
	ID             string    `json:"id"`
	MediaServerID  string    `json:"media_server_id"`
	ServerSeriesID string    `json:"server_series_id"`
	LibraryID      string    `json:"library_id"`
	Title          string    `json:"title"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type SeriesRepo struct {
	pool *pgxpool.Pool
}

func NewSeriesRepo(pool *pgxpool.Pool) *SeriesRepo {
	return &SeriesRepo{pool: pool}
}

func (r *SeriesRepo) Upsert(ctx context.Context, s *Series) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO series (id, media_server_id, server_series_id, library_id, title, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		 ON CONFLICT (media_server_id, server_series_id) DO UPDATE SET
		   title = EXCLUDED.title,
		   library_id = EXCLUDED.library_id,
		   updated_at = EXCLUDED.updated_at`,
		s.ID, s.MediaServerID, s.ServerSeriesID, s.LibraryID, s.Title, s.Active, time.Now())
	return err
}

func (r *SeriesRepo) List(ctx context.Context) ([]Series, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, media_server_id, server_series_id, library_id, title, active, created_at, updated_at FROM series ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	defer rows.Close()

	var series []Series
	for rows.Next() {
		var s Series
		if err := rows.Scan(&s.ID, &s.MediaServerID, &s.ServerSeriesID, &s.LibraryID, &s.Title, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan series: %w", err)
		}
		series = append(series, s)
	}
	return series, nil
}

func (r *SeriesRepo) GetByID(ctx context.Context, id string) (*Series, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, media_server_id, server_series_id, library_id, title, active, created_at, updated_at FROM series WHERE id = $1`, id)
	var s Series
	if err := row.Scan(&s.ID, &s.MediaServerID, &s.ServerSeriesID, &s.LibraryID, &s.Title, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("series not found")
		}
		return nil, fmt.Errorf("get series: %w", err)
	}
	return &s, nil
}

func (r *SeriesRepo) SetActive(ctx context.Context, id string, active bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE series SET active = $2, updated_at = now() WHERE id = $1`, id, active)
	return err
}

func (r *SeriesRepo) ListActive(ctx context.Context) ([]Series, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, media_server_id, server_series_id, library_id, title, active, created_at, updated_at FROM series WHERE active = true ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("list active series: %w", err)
	}
	defer rows.Close()

	var series []Series
	for rows.Next() {
		var s Series
		if err := rows.Scan(&s.ID, &s.MediaServerID, &s.ServerSeriesID, &s.LibraryID, &s.Title, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan series: %w", err)
		}
		series = append(series, s)
	}
	return series, nil
}

// --- Episode ---

type Episode struct {
	ID              string    `json:"id"`
	SeriesID        string    `json:"series_id"`
	ServerEpisodeID string    `json:"server_episode_id"`
	SeasonNumber    int       `json:"season_number"`
	EpisodeNumber   int       `json:"episode_number"`
	AbsoluteOrder   int       `json:"absolute_order"`
	Title           string    `json:"title"`
	Duration        int       `json:"duration"`
	Rating          float64   `json:"rating"`
	AirDate         string    `json:"air_date"`
	CreatedAt       time.Time `json:"created_at"`
}

type EpisodeRepo struct {
	pool *pgxpool.Pool
}

func NewEpisodeRepo(pool *pgxpool.Pool) *EpisodeRepo {
	return &EpisodeRepo{pool: pool}
}

func (r *EpisodeRepo) Upsert(ctx context.Context, e *Episode) error {
	var airDate interface{} = e.AirDate
	if e.AirDate == "" {
		airDate = nil
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO episodes (id, series_id, server_episode_id, season_number, episode_number, absolute_order, title, duration_seconds, rating, originally_available_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (series_id, server_episode_id) DO UPDATE SET
		   season_number = EXCLUDED.season_number,
		   episode_number = EXCLUDED.episode_number,
		   absolute_order = EXCLUDED.absolute_order,
		   title = EXCLUDED.title,
		   duration_seconds = EXCLUDED.duration_seconds,
		   rating = EXCLUDED.rating,
		   originally_available_at = EXCLUDED.originally_available_at`,
		e.ID, e.SeriesID, e.ServerEpisodeID, e.SeasonNumber, e.EpisodeNumber, e.AbsoluteOrder, e.Title, e.Duration, e.Rating, airDate, time.Now())
	return err
}

func (r *EpisodeRepo) ListBySeries(ctx context.Context, seriesID string) ([]Episode, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, series_id, server_episode_id, season_number, episode_number, absolute_order, title, duration_seconds, rating, COALESCE(originally_available_at::text, ''), created_at FROM episodes WHERE series_id = $1 ORDER BY absolute_order`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var e Episode
		if err := rows.Scan(&e.ID, &e.SeriesID, &e.ServerEpisodeID, &e.SeasonNumber, &e.EpisodeNumber, &e.AbsoluteOrder, &e.Title, &e.Duration, &e.Rating, &e.AirDate, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		episodes = append(episodes, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate episodes: %w", err)
	}
	return episodes, nil
}

func (r *EpisodeRepo) GetByID(ctx context.Context, id string) (*Episode, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, series_id, server_episode_id, season_number, episode_number, absolute_order, title, duration_seconds, rating, COALESCE(originally_available_at::text, ''), created_at FROM episodes WHERE id = $1`, id)
	var e Episode
	if err := row.Scan(&e.ID, &e.SeriesID, &e.ServerEpisodeID, &e.SeasonNumber, &e.EpisodeNumber, &e.AbsoluteOrder, &e.Title, &e.Duration, &e.Rating, &e.AirDate, &e.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("episode not found")
		}
		return nil, fmt.Errorf("get episode: %w", err)
	}
	return &e, nil
}

func (r *EpisodeRepo) GetByServerEpisodeID(ctx context.Context, mediaServerID, serverEpisodeID string) (*Episode, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT e.id, e.series_id, e.server_episode_id, e.season_number, e.episode_number, e.absolute_order, e.title, e.duration_seconds, e.rating, COALESCE(e.originally_available_at::text, ''), e.created_at
		 FROM episodes e
		 JOIN series s ON s.id = e.series_id
		 WHERE s.media_server_id = $1 AND e.server_episode_id = $2`, mediaServerID, serverEpisodeID)
	var e Episode
	if err := row.Scan(&e.ID, &e.SeriesID, &e.ServerEpisodeID, &e.SeasonNumber, &e.EpisodeNumber, &e.AbsoluteOrder, &e.Title, &e.Duration, &e.Rating, &e.AirDate, &e.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("episode not found")
		}
		return nil, fmt.Errorf("get episode by server ID: %w", err)
	}
	return &e, nil
}

func (r *EpisodeRepo) CountBySeries(ctx context.Context, seriesID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM episodes WHERE series_id = $1`, seriesID).Scan(&count)
	return count, err
}

// --- Progress ---

type SeriesProgress struct {
	SeriesID             string     `json:"series_id"`
	LastWatchedEpisodeID *string    `json:"last_watched_episode_id"`
	NextEpisodeID        *string    `json:"next_episode_id"`
	NextPosition         *int       `json:"next_position"`
	SynchronizedAt       *time.Time `json:"synchronized_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ProgressRepo struct {
	pool *pgxpool.Pool
}

func NewProgressRepo(pool *pgxpool.Pool) *ProgressRepo {
	return &ProgressRepo{pool: pool}
}

func (r *ProgressRepo) Upsert(ctx context.Context, p *SeriesProgress) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO series_progress (series_id, last_watched_episode_id, next_episode_id, next_position, synchronized_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (series_id) DO UPDATE SET
		   last_watched_episode_id = COALESCE(EXCLUDED.last_watched_episode_id, series_progress.last_watched_episode_id),
		   next_episode_id = EXCLUDED.next_episode_id,
		   next_position = EXCLUDED.next_position,
		   synchronized_at = EXCLUDED.synchronized_at,
		   updated_at = EXCLUDED.updated_at`,
		p.SeriesID, p.LastWatchedEpisodeID, p.NextEpisodeID, p.NextPosition, p.SynchronizedAt, time.Now())
	return err
}

func (r *ProgressRepo) GetBySeries(ctx context.Context, seriesID string) (*SeriesProgress, error) {
	row := r.pool.QueryRow(ctx, `SELECT series_id, last_watched_episode_id, next_episode_id, next_position, synchronized_at, updated_at FROM series_progress WHERE series_id = $1`, seriesID)
	var p SeriesProgress
	if err := row.Scan(&p.SeriesID, &p.LastWatchedEpisodeID, &p.NextEpisodeID, &p.NextPosition, &p.SynchronizedAt, &p.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("progress not found")
		}
		return nil, fmt.Errorf("get progress: %w", err)
	}
	return &p, nil
}

func (r *ProgressRepo) Advance(ctx context.Context, seriesID string, watchedEpisodeID string, nextEpisodeID *string, nextPosition *int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE series_progress SET
		   last_watched_episode_id = $2,
		   next_episode_id = $3,
		   next_position = $4,
		   updated_at = now()
		 WHERE series_id = $1`,
		seriesID, watchedEpisodeID, nextEpisodeID, nextPosition)
	return err
}

// --- Rotation Profile ---

type RotationProfile struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Enabled       bool            `json:"enabled"`
	Configuration json.RawMessage `json:"configuration"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type PolicyRepo struct {
	pool *pgxpool.Pool
}

func NewPolicyRepo(pool *pgxpool.Pool) *PolicyRepo {
	return &PolicyRepo{pool: pool}
}

func (r *PolicyRepo) Create(ctx context.Context, id, name string, config json.RawMessage) (*RotationProfile, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rotation_profiles (id, name, enabled, configuration, created_at, updated_at)
		 VALUES ($1, $2, true, $3, now(), now())`,
		id, name, config)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}
	return &RotationProfile{ID: id, Name: name, Enabled: true, Configuration: config, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (r *PolicyRepo) Update(ctx context.Context, id, name string, config json.RawMessage) (*RotationProfile, error) {
	_, err := r.pool.Exec(ctx,
		`UPDATE rotation_profiles SET name = $2, configuration = $3, updated_at = now() WHERE id = $1`,
		id, name, config)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return &RotationProfile{ID: id, Name: name, Configuration: config, UpdatedAt: time.Now()}, nil
}

func (r *PolicyRepo) List(ctx context.Context) ([]RotationProfile, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, enabled, configuration, created_at, updated_at FROM rotation_profiles ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	var profiles []RotationProfile
	for rows.Next() {
		var p RotationProfile
		if err := rows.Scan(&p.ID, &p.Name, &p.Enabled, &p.Configuration, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func (r *PolicyRepo) GetByID(ctx context.Context, id string) (*RotationProfile, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, name, enabled, configuration, created_at, updated_at FROM rotation_profiles WHERE id = $1`, id)
	var p RotationProfile
	if err := row.Scan(&p.ID, &p.Name, &p.Enabled, &p.Configuration, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return &p, nil
}

func (r *PolicyRepo) GetFirstEnabled(ctx context.Context) (*RotationProfile, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, name, enabled, configuration, created_at, updated_at FROM rotation_profiles WHERE enabled = true ORDER BY created_at LIMIT 1`)
	var p RotationProfile
	if err := row.Scan(&p.ID, &p.Name, &p.Enabled, &p.Configuration, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no enabled profile found")
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return &p, nil
}

// --- Rotation ---

type Rotation struct {
	ID               string     `json:"id"`
	ProfileID        string     `json:"profile_id"`
	Status           string     `json:"status"`
	GeneratedAt      time.Time  `json:"generated_at"`
	PublishedAt      *time.Time `json:"published_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	AvailableMinutes int        `json:"available_minutes"`
	RandomSeed       int64      `json:"random_seed"`
}

type RotationItem struct {
	ID           string          `json:"id"`
	RotationID   string          `json:"rotation_id"`
	Position     int             `json:"position"`
	SeriesID     string          `json:"series_id"`
	EpisodeID    string          `json:"episode_id"`
	SlotKind     string          `json:"slot_kind"`
	Score        float64         `json:"score"`
	ScoreDetails json.RawMessage `json:"score_details"`
	Status       string          `json:"status"`
}

type RotationRepo struct {
	pool *pgxpool.Pool
}

func NewRotationRepo(pool *pgxpool.Pool) *RotationRepo {
	return &RotationRepo{pool: pool}
}

func (r *RotationRepo) Create(ctx context.Context, id, profileID, status string, seed int64, budgetMinutes int) (*Rotation, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rotations (id, profile_id, status, generated_at, random_seed, available_minutes)
		 VALUES ($1, $2, $3, now(), $4, $5)`,
		id, profileID, status, seed, budgetMinutes)
	if err != nil {
		return nil, fmt.Errorf("create rotation: %w", err)
	}
	return &Rotation{ID: id, ProfileID: profileID, Status: status, GeneratedAt: time.Now(), RandomSeed: seed, AvailableMinutes: budgetMinutes}, nil
}

func (r *RotationRepo) AddItem(ctx context.Context, item *RotationItem) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rotation_items (id, rotation_id, position, series_id, episode_id, slot_kind, score, score_details, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		item.ID, item.RotationID, item.Position, item.SeriesID, item.EpisodeID, item.SlotKind, item.Score, item.ScoreDetails, item.Status)
	return err
}

func (r *RotationRepo) GetCurrent(ctx context.Context) (*Rotation, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, profile_id, status, generated_at, published_at, completed_at, available_minutes, random_seed
		 FROM rotations WHERE status IN ('draft', 'published')
		 ORDER BY generated_at DESC LIMIT 1`)
	var rot Rotation
	if err := row.Scan(&rot.ID, &rot.ProfileID, &rot.Status, &rot.GeneratedAt, &rot.PublishedAt, &rot.CompletedAt, &rot.AvailableMinutes, &rot.RandomSeed); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no current rotation")
		}
		return nil, fmt.Errorf("get current rotation: %w", err)
	}
	return &rot, nil
}

func (r *RotationRepo) GetByID(ctx context.Context, id string) (*Rotation, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, profile_id, status, generated_at, published_at, completed_at, available_minutes, random_seed
		 FROM rotations WHERE id = $1`, id)
	var rot Rotation
	if err := row.Scan(&rot.ID, &rot.ProfileID, &rot.Status, &rot.GeneratedAt, &rot.PublishedAt, &rot.CompletedAt, &rot.AvailableMinutes, &rot.RandomSeed); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("rotation not found")
		}
		return nil, fmt.Errorf("get rotation: %w", err)
	}
	return &rot, nil
}

func (r *RotationRepo) SetStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE rotations SET status = $2, published_at = CASE WHEN $2 = 'published' THEN now() ELSE published_at END, completed_at = CASE WHEN $2 = 'completed' THEN now() ELSE completed_at END WHERE id = $1`, id, status)
	return err
}

func (r *RotationRepo) ListItems(ctx context.Context, rotationID string) ([]RotationItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, rotation_id, position, series_id, episode_id, slot_kind, score, score_details, status
		 FROM rotation_items WHERE rotation_id = $1 ORDER BY position`, rotationID)
	if err != nil {
		return nil, fmt.Errorf("list rotation items: %w", err)
	}
	defer rows.Close()

	var items []RotationItem
	for rows.Next() {
		var item RotationItem
		if err := rows.Scan(&item.ID, &item.RotationID, &item.Position, &item.SeriesID, &item.EpisodeID, &item.SlotKind, &item.Score, &item.ScoreDetails, &item.Status); err != nil {
			return nil, fmt.Errorf("scan rotation item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *RotationRepo) UpdateItemStatus(ctx context.Context, itemID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE rotation_items SET status = $2 WHERE id = $1`, itemID, status)
	return err
}

// --- Queue Playlist Binding ---

type QueuePlaylistBinding struct {
	ID               string     `json:"id"`
	PlaylistID       string     `json:"playlist_id"`
	MediaServerID    string     `json:"media_server_id"`
	ServerPlaylistID *string    `json:"server_playlist_id"`
	PlaylistName     string     `json:"playlist_name"`
	SynchronizedAt   *time.Time `json:"synchronized_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

type QueueBindingRepo struct {
	pool *pgxpool.Pool
}

func NewQueueBindingRepo(pool *pgxpool.Pool) *QueueBindingRepo {
	return &QueueBindingRepo{pool: pool}
}

func (r *QueueBindingRepo) Upsert(ctx context.Context, binding *QueuePlaylistBinding) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO queue_playlist_bindings (id, playlist_id, media_server_id, server_playlist_id, playlist_name, synchronized_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, now())
		 ON CONFLICT (playlist_id) DO UPDATE SET
		   server_playlist_id = EXCLUDED.server_playlist_id,
		   playlist_name = EXCLUDED.playlist_name,
		   synchronized_at = EXCLUDED.synchronized_at`,
		binding.ID, binding.PlaylistID, binding.MediaServerID, binding.ServerPlaylistID, binding.PlaylistName, binding.SynchronizedAt)
	return err
}

func (r *QueueBindingRepo) GetByPlaylist(ctx context.Context, playlistID string) (*QueuePlaylistBinding, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, playlist_id, media_server_id, server_playlist_id, playlist_name, synchronized_at, created_at
		 FROM queue_playlist_bindings WHERE playlist_id = $1`, playlistID)
	var b QueuePlaylistBinding
	if err := row.Scan(&b.ID, &b.PlaylistID, &b.MediaServerID, &b.ServerPlaylistID, &b.PlaylistName, &b.SynchronizedAt, &b.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("binding not found")
		}
		return nil, fmt.Errorf("get queue binding: %w", err)
	}
	return &b, nil
}

// --- Rotation Profile Bindings (for legacy rotation profiles) ---

type PlaylistBinding struct {
	ID                string     `json:"id"`
	MediaServerID     string     `json:"media_server_id"`
	RotationProfileID string     `json:"rotation_profile_id"`
	ServerPlaylistID  *string    `json:"server_playlist_id"`
	PlaylistName      string     `json:"playlist_name"`
	SynchronizedAt    *time.Time `json:"synchronized_at"`
}

type BindingRepo struct {
	pool *pgxpool.Pool
}

func NewBindingRepo(pool *pgxpool.Pool) *BindingRepo {
	return &BindingRepo{pool: pool}
}

func (r *BindingRepo) Upsert(ctx context.Context, binding *PlaylistBinding) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO playlist_bindings (id, media_server_id, rotation_profile_id, server_playlist_id, playlist_name, synchronized_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (media_server_id, rotation_profile_id) DO UPDATE SET
		   server_playlist_id = EXCLUDED.server_playlist_id,
		   playlist_name = EXCLUDED.playlist_name,
		   synchronized_at = EXCLUDED.synchronized_at`,
		binding.ID, binding.MediaServerID, binding.RotationProfileID, binding.ServerPlaylistID, binding.PlaylistName, binding.SynchronizedAt)
	return err
}

func (r *BindingRepo) GetByProfile(ctx context.Context, profileID string) (*PlaylistBinding, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, media_server_id, rotation_profile_id, server_playlist_id, playlist_name, synchronized_at
		 FROM playlist_bindings WHERE rotation_profile_id = $1`, profileID)
	var b PlaylistBinding
	if err := row.Scan(&b.ID, &b.MediaServerID, &b.RotationProfileID, &b.ServerPlaylistID, &b.PlaylistName, &b.SynchronizedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("binding not found")
		}
		return nil, fmt.Errorf("get binding: %w", err)
	}
	return &b, nil
}
