package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Playlist struct {
	ID               string    `json:"id"`
	MediaServerID    string    `json:"media_server_id"`
	Name             string    `json:"name"`
	PlexPlaylistName string    `json:"plex_playlist_name"`
	QueueTargetCount int       `json:"queue_target_count"`
	CycleCursor      int       `json:"cycle_cursor"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PlaylistSlot struct {
	ID         string `json:"id"`
	PlaylistID string `json:"playlist_id"`
	Position   int    `json:"position"`
	SlotType   string `json:"slot_type"`
}

type PlaylistSeries struct {
	ID            string    `json:"id"`
	PlaylistID    string    `json:"playlist_id"`
	SeriesID      string    `json:"series_id"`
	Mode          string    `json:"mode"`
	ShowProfileID *string   `json:"show_profile_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type PlaylistProgress struct {
	ID                   string    `json:"id"`
	PlaylistSeriesID     string    `json:"playlist_series_id"`
	NextEpisodeID        *string   `json:"next_episode_id"`
	NextPosition         *int      `json:"next_position"`
	LastWatchedEpisodeID *string   `json:"last_watched_episode_id"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type PlaylistHistory struct {
	ID               string    `json:"id"`
	PlaylistSeriesID string    `json:"playlist_series_id"`
	EpisodeID        string    `json:"episode_id"`
	PlayedAt         time.Time `json:"played_at"`
}

type PlaylistQueueItem struct {
	ID               string    `json:"id"`
	PlaylistID       string    `json:"playlist_id"`
	CycleIndex       int       `json:"cycle_index"`
	SlotPosition     int       `json:"slot_position"`
	SlotType         string    `json:"slot_type"`
	SeriesID         string    `json:"series_id"`
	PlaylistSeriesID *string   `json:"playlist_series_id"`
	EpisodeID        string    `json:"episode_id"`
	Position         int       `json:"position"`
	Score            *float64  `json:"score"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type PlaylistRepo struct {
	pool *pgxpool.Pool
}

func NewPlaylistRepo(pool *pgxpool.Pool) *PlaylistRepo {
	return &PlaylistRepo{pool: pool}
}

func (r *PlaylistRepo) Create(ctx context.Context, p *Playlist) (*Playlist, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO playlists (id, media_server_id, name, plex_playlist_name, queue_target_count, cycle_cursor, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())`,
		p.ID, p.MediaServerID, p.Name, p.PlexPlaylistName, p.QueueTargetCount, p.CycleCursor, p.Enabled)
	if err != nil {
		return nil, fmt.Errorf("create playlist: %w", err)
	}
	return p, nil
}

func (r *PlaylistRepo) GetByID(ctx context.Context, id string) (*Playlist, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, media_server_id, name, plex_playlist_name, queue_target_count, cycle_cursor, enabled, created_at, updated_at
		 FROM playlists WHERE id = $1`, id)
	var p Playlist
	if err := row.Scan(&p.ID, &p.MediaServerID, &p.Name, &p.PlexPlaylistName, &p.QueueTargetCount, &p.CycleCursor, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("playlist not found")
		}
		return nil, fmt.Errorf("get playlist: %w", err)
	}
	return &p, nil
}

func (r *PlaylistRepo) List(ctx context.Context) ([]Playlist, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, media_server_id, name, plex_playlist_name, queue_target_count, cycle_cursor, enabled, created_at, updated_at
		 FROM playlists ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	defer rows.Close()

	var playlists []Playlist
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.MediaServerID, &p.Name, &p.PlexPlaylistName, &p.QueueTargetCount, &p.CycleCursor, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan playlist: %w", err)
		}
		playlists = append(playlists, p)
	}
	return playlists, nil
}

func (r *PlaylistRepo) Update(ctx context.Context, p *Playlist) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE playlists SET name = $2, plex_playlist_name = $3, queue_target_count = $4, enabled = $5, updated_at = now()
		 WHERE id = $1`,
		p.ID, p.Name, p.PlexPlaylistName, p.QueueTargetCount, p.Enabled)
	return err
}

func (r *PlaylistRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, id)
	return err
}

func (r *PlaylistRepo) IncrementCursor(ctx context.Context, id string, delta int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE playlists SET cycle_cursor = cycle_cursor + $2, updated_at = now() WHERE id = $1`,
		id, delta)
	return err
}

func (r *PlaylistRepo) GetEnabledForServer(ctx context.Context, serverID string) ([]Playlist, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, media_server_id, name, plex_playlist_name, queue_target_count, cycle_cursor, enabled, created_at, updated_at
		 FROM playlists WHERE media_server_id = $1 AND enabled = true ORDER BY created_at`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list enabled playlists: %w", err)
	}
	defer rows.Close()

	var playlists []Playlist
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.MediaServerID, &p.Name, &p.PlexPlaylistName, &p.QueueTargetCount, &p.CycleCursor, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan playlist: %w", err)
		}
		playlists = append(playlists, p)
	}
	return playlists, nil
}

// --- Slots ---

func (r *PlaylistRepo) SetSlots(ctx context.Context, playlistID string, slots []PlaylistSlot) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM playlist_slots WHERE playlist_id = $1`, playlistID); err != nil {
		return fmt.Errorf("delete slots: %w", err)
	}

	for _, s := range slots {
		if _, err := tx.Exec(ctx,
			`INSERT INTO playlist_slots (id, playlist_id, position, slot_type) VALUES ($1, $2, $3, $4)`,
			s.ID, s.PlaylistID, s.Position, s.SlotType); err != nil {
			return fmt.Errorf("insert slot: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PlaylistRepo) ListSlots(ctx context.Context, playlistID string) ([]PlaylistSlot, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, playlist_id, position, slot_type FROM playlist_slots WHERE playlist_id = $1 ORDER BY position`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list slots: %w", err)
	}
	defer rows.Close()

	var slots []PlaylistSlot
	for rows.Next() {
		var s PlaylistSlot
		if err := rows.Scan(&s.ID, &s.PlaylistID, &s.Position, &s.SlotType); err != nil {
			return nil, fmt.Errorf("scan slot: %w", err)
		}
		slots = append(slots, s)
	}
	return slots, nil
}

// --- Series membership ---

type PlaylistSeriesInput struct {
	SeriesID      string
	Mode          string
	ShowProfileID *string
}

func (r *PlaylistRepo) SetPlaylistSeries(ctx context.Context, playlistID string, newSeries []PlaylistSeriesInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx,
		`SELECT id, playlist_id, series_id, mode, show_profile_id, created_at FROM playlist_series WHERE playlist_id = $1 ORDER BY created_at`, playlistID)
	if err != nil {
		return fmt.Errorf("list current: %w", err)
	}
	defer rows.Close()

	var current []PlaylistSeries
	for rows.Next() {
		var member PlaylistSeries
		if err := rows.Scan(&member.ID, &member.PlaylistID, &member.SeriesID, &member.Mode, &member.ShowProfileID, &member.CreatedAt); err != nil {
			return fmt.Errorf("scan current member: %w", err)
		}
		current = append(current, member)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate current members: %w", err)
	}

	currentMap := make(map[string]PlaylistSeries) // seriesID -> member
	for _, m := range current {
		currentMap[m.SeriesID] = m
	}

	newSet := make(map[string]struct{})
	for _, ns := range newSeries {
		newSet[ns.SeriesID] = struct{}{}
	}

	// Delete removed members — nullify queue refs first, then delete
	for _, m := range current {
		if _, keep := newSet[m.SeriesID]; !keep {
			if _, err := tx.Exec(ctx, `UPDATE playlist_queue_items SET playlist_series_id = NULL WHERE playlist_series_id = $1`, m.ID); err != nil {
				return fmt.Errorf("nullify queue refs: %w", err)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM playlist_series WHERE id = $1`, m.ID); err != nil {
				return fmt.Errorf("delete member: %w", err)
			}
		}
	}

	// Add new members, update selection behavior on existing members.
	for _, ns := range newSeries {
		if existing, ok := currentMap[ns.SeriesID]; ok {
			profileChanged := !sameOptionalString(existing.ShowProfileID, ns.ShowProfileID)
			if existing.Mode != ns.Mode || profileChanged {
				if _, err := tx.Exec(ctx, `UPDATE playlist_series SET mode = $1, show_profile_id = $2 WHERE id = $3`, ns.Mode, ns.ShowProfileID, existing.ID); err != nil {
					return fmt.Errorf("update playlist series: %w", err)
				}
				// Preserve a currently watched episode, but replace queue entries whose
				// eligibility was determined under the former mode or profile.
				if _, err := tx.Exec(ctx,
					`UPDATE playlist_queue_items SET status = 'skipped'
					 WHERE playlist_id = $1 AND series_id = $2 AND status IN ('pending', 'pushed')`, playlistID, ns.SeriesID); err != nil {
					return fmt.Errorf("skip queue entries for selection change: %w", err)
				}
			}
		} else {
			if _, err := tx.Exec(ctx,
				`INSERT INTO playlist_series (id, playlist_id, series_id, mode, show_profile_id, created_at) VALUES (gen_random_uuid(), $1, $2, $3, $4, now())`,
				playlistID, ns.SeriesID, ns.Mode, ns.ShowProfileID); err != nil {
				return fmt.Errorf("insert member: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *PlaylistRepo) ListSeries(ctx context.Context, playlistID string) ([]PlaylistSeries, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, playlist_id, series_id, mode, show_profile_id, created_at FROM playlist_series WHERE playlist_id = $1 ORDER BY created_at`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list playlist series: %w", err)
	}
	defer rows.Close()

	var series []PlaylistSeries
	for rows.Next() {
		var ps PlaylistSeries
		if err := rows.Scan(&ps.ID, &ps.PlaylistID, &ps.SeriesID, &ps.Mode, &ps.ShowProfileID, &ps.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan playlist series: %w", err)
		}
		series = append(series, ps)
	}
	return series, nil
}

func (r *PlaylistRepo) GetPlaylistSeriesByID(ctx context.Context, id string) (*PlaylistSeries, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, playlist_id, series_id, mode, show_profile_id, created_at FROM playlist_series WHERE id = $1`, id)
	var ps PlaylistSeries
	if err := row.Scan(&ps.ID, &ps.PlaylistID, &ps.SeriesID, &ps.Mode, &ps.ShowProfileID, &ps.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("playlist series not found")
		}
		return nil, fmt.Errorf("get playlist series: %w", err)
	}
	return &ps, nil
}

func sameOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// --- Progress ---

func (r *PlaylistRepo) GetProgress(ctx context.Context, playlistSeriesID string) (*PlaylistProgress, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, playlist_series_id, next_episode_id, next_position, last_watched_episode_id, updated_at
		 FROM playlist_series_progress WHERE playlist_series_id = $1`, playlistSeriesID)
	var p PlaylistProgress
	if err := row.Scan(&p.ID, &p.PlaylistSeriesID, &p.NextEpisodeID, &p.NextPosition, &p.LastWatchedEpisodeID, &p.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get progress: %w", err)
	}
	return &p, nil
}

func (r *PlaylistRepo) UpsertProgress(ctx context.Context, p *PlaylistProgress) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO playlist_series_progress (id, playlist_series_id, next_episode_id, next_position, last_watched_episode_id, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now())
		 ON CONFLICT (playlist_series_id) DO UPDATE SET
		   next_episode_id = EXCLUDED.next_episode_id,
		   next_position = EXCLUDED.next_position,
		   last_watched_episode_id = COALESCE(EXCLUDED.last_watched_episode_id, playlist_series_progress.last_watched_episode_id),
		   updated_at = now()`,
		p.ID, p.PlaylistSeriesID, p.NextEpisodeID, p.NextPosition, p.LastWatchedEpisodeID)
	return err
}

func (r *PlaylistRepo) AdvanceProgress(ctx context.Context, playlistSeriesID string, watchedEpisodeID string, nextEpisodeID *string, nextPosition *int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE playlist_series_progress SET
		   last_watched_episode_id = $2,
		   next_episode_id = $3,
		   next_position = $4,
		   updated_at = now()
		 WHERE playlist_series_id = $1`,
		playlistSeriesID, watchedEpisodeID, nextEpisodeID, nextPosition)
	return err
}

// --- History ---

func (r *PlaylistRepo) AddHistory(ctx context.Context, playlistSeriesID, episodeID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO playlist_series_history (id, playlist_series_id, episode_id, played_at)
		 VALUES (gen_random_uuid(), $1, $2, now())
		 ON CONFLICT DO NOTHING`,
		playlistSeriesID, episodeID)
	return err
}

func (r *PlaylistRepo) ListHistory(ctx context.Context, playlistSeriesID string) ([]PlaylistHistory, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, playlist_series_id, episode_id, played_at
		 FROM playlist_series_history WHERE playlist_series_id = $1 ORDER BY played_at`, playlistSeriesID)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var history []PlaylistHistory
	for rows.Next() {
		var h PlaylistHistory
		if err := rows.Scan(&h.ID, &h.PlaylistSeriesID, &h.EpisodeID, &h.PlayedAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		history = append(history, h)
	}
	return history, nil
}

func (r *PlaylistRepo) HistoryEpisodeIDs(ctx context.Context, playlistSeriesID string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT episode_id FROM playlist_series_history WHERE playlist_series_id = $1`, playlistSeriesID)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		ids[id] = true
	}
	return ids, nil
}

// --- Queue items ---

func (r *PlaylistRepo) AddQueueItem(ctx context.Context, item *PlaylistQueueItem) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO playlist_queue_items (id, playlist_id, cycle_index, slot_position, slot_type, series_id, playlist_series_id, episode_id, position, score, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())`,
		item.ID, item.PlaylistID, item.CycleIndex, item.SlotPosition, item.SlotType, item.SeriesID, item.PlaylistSeriesID, item.EpisodeID, item.Position, item.Score, item.Status)
	return err
}

func (r *PlaylistRepo) CountPendingQueueItems(ctx context.Context, playlistID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM playlist_queue_items
		 WHERE playlist_id = $1 AND status IN ('pending', 'pushed', 'watching')`, playlistID).Scan(&count)
	return count, err
}

func (r *PlaylistRepo) ListQueueItems(ctx context.Context, playlistID string) ([]PlaylistQueueItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, playlist_id, cycle_index, slot_position, slot_type, series_id, playlist_series_id, episode_id, position, score, status, created_at
		 FROM playlist_queue_items WHERE playlist_id = $1 ORDER BY position`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list queue items: %w", err)
	}
	defer rows.Close()

	var items []PlaylistQueueItem
	for rows.Next() {
		var qi PlaylistQueueItem
		if err := rows.Scan(&qi.ID, &qi.PlaylistID, &qi.CycleIndex, &qi.SlotPosition, &qi.SlotType, &qi.SeriesID, &qi.PlaylistSeriesID, &qi.EpisodeID, &qi.Position, &qi.Score, &qi.Status, &qi.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan queue item: %w", err)
		}
		items = append(items, qi)
	}
	return items, nil
}

func (r *PlaylistRepo) ListPendingQueueItems(ctx context.Context, playlistID string) ([]PlaylistQueueItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, playlist_id, cycle_index, slot_position, slot_type, series_id, playlist_series_id, episode_id, position, score, status, created_at
		 FROM playlist_queue_items WHERE playlist_id = $1 AND status IN ('pending', 'pushed', 'watching') ORDER BY position`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list pending queue items: %w", err)
	}
	defer rows.Close()

	var items []PlaylistQueueItem
	for rows.Next() {
		var qi PlaylistQueueItem
		if err := rows.Scan(&qi.ID, &qi.PlaylistID, &qi.CycleIndex, &qi.SlotPosition, &qi.SlotType, &qi.SeriesID, &qi.PlaylistSeriesID, &qi.EpisodeID, &qi.Position, &qi.Score, &qi.Status, &qi.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan queue item: %w", err)
		}
		items = append(items, qi)
	}
	return items, nil
}

func (r *PlaylistRepo) MarkPushed(ctx context.Context, playlistID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE playlist_queue_items SET status = 'pushed'
		 WHERE playlist_id = $1 AND status = 'pending'`, playlistID)
	return err
}

func (r *PlaylistRepo) UpdateItemStatus(ctx context.Context, itemID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE playlist_queue_items SET status = $2 WHERE id = $1`, itemID, status)
	return err
}

func (r *PlaylistRepo) GetQueueItemByID(ctx context.Context, itemID string) (*PlaylistQueueItem, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, playlist_id, cycle_index, slot_position, slot_type, series_id, playlist_series_id, episode_id, position, score, status, created_at
		 FROM playlist_queue_items WHERE id = $1`, itemID)
	var qi PlaylistQueueItem
	if err := row.Scan(&qi.ID, &qi.PlaylistID, &qi.CycleIndex, &qi.SlotPosition, &qi.SlotType, &qi.SeriesID, &qi.PlaylistSeriesID, &qi.EpisodeID, &qi.Position, &qi.Score, &qi.Status, &qi.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("queue item not found")
		}
		return nil, fmt.Errorf("get queue item: %w", err)
	}
	return &qi, nil
}

func (r *PlaylistRepo) DeleteWatchedQueueItems(ctx context.Context, playlistID string) (int, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM playlist_queue_items
		 WHERE playlist_id = $1 AND status = 'watched'`, playlistID)
	if err != nil {
		return 0, fmt.Errorf("delete watched items: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *PlaylistRepo) DeleteWatchedQueueItem(ctx context.Context, itemID string) (int, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM playlist_queue_items WHERE id = $1 AND status = 'watched'`, itemID)
	if err != nil {
		return 0, fmt.Errorf("delete watched item: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *PlaylistRepo) ClearQueue(ctx context.Context, playlistID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin clear queue transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM playlist_queue_items WHERE playlist_id = $1`, playlistID); err != nil {
		return fmt.Errorf("delete queue items: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE playlists SET cycle_cursor = 0, updated_at = now() WHERE id = $1`, playlistID); err != nil {
		return fmt.Errorf("reset queue cursor: %w", err)
	}
	return tx.Commit(ctx)
}

// --- Max position for ordering ---

func (r *PlaylistRepo) MaxQueuePosition(ctx context.Context, playlistID string) (int, error) {
	var maxPos int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) FROM playlist_queue_items WHERE playlist_id = $1`, playlistID).Scan(&maxPos)
	return maxPos, err
}

func (r *PlaylistRepo) EnsureDefaultSlot(ctx context.Context, playlistID string) error {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM playlist_slots WHERE playlist_id = $1`, playlistID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO playlist_slots (id, playlist_id, position, slot_type) VALUES (gen_random_uuid(), $1, 0, 'any')`, playlistID)
	return err
}

// GetQueuedEpisodeIDsForSeries returns episode IDs currently queued (pending/pushed) for a series within a playlist.
func (r *PlaylistRepo) GetQueuedEpisodeIDsForSeries(ctx context.Context, playlistID, seriesID string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT episode_id FROM playlist_queue_items
		 WHERE playlist_id = $1 AND series_id = $2 AND status IN ('pending', 'pushed', 'watching')`, playlistID, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, nil
}

func (r *PlaylistRepo) GetHistoryCount(ctx context.Context, playlistSeriesID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM playlist_series_history WHERE playlist_series_id = $1`, playlistSeriesID).Scan(&count)
	return count, err
}

func (r *PlaylistRepo) SkipPendingForSeries(ctx context.Context, playlistID, seriesID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE playlist_queue_items SET status = 'skipped'
		 WHERE playlist_id = $1 AND series_id = $2 AND status IN ('pending', 'pushed', 'watching')`,
		playlistID, seriesID)
	return err
}
