package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andrew/rotator/internal/media"
	"github.com/andrew/rotator/internal/media/plex"
	"github.com/andrew/rotator/internal/repository"
	"github.com/andrew/rotator/internal/rotation"
	"github.com/google/uuid"
)

type Service struct {
	playlistName string
	plexClient   *plex.Client

	serverRepo       *repository.ServerRepo
	seriesRepo       *repository.SeriesRepo
	episodeRepo      *repository.EpisodeRepo
	progressRepo     *repository.ProgressRepo
	policyRepo       *repository.PolicyRepo
	rotationRepo     *repository.RotationRepo
	bindingRepo      *repository.BindingRepo
	playlistRepo     *repository.PlaylistRepo
	queueBindingRepo *repository.QueueBindingRepo
	showProfileRepo  *repository.ShowProfileRepo

	engine          *rotation.Engine
	playlistLocksMu sync.Mutex
	playlistLocks   map[string]*sync.Mutex
}

func New(
	playlistName string,
	plexClient *plex.Client,
	serverRepo *repository.ServerRepo,
	seriesRepo *repository.SeriesRepo,
	episodeRepo *repository.EpisodeRepo,
	progressRepo *repository.ProgressRepo,
	policyRepo *repository.PolicyRepo,
	rotationRepo *repository.RotationRepo,
	bindingRepo *repository.BindingRepo,
	playlistRepo *repository.PlaylistRepo,
	queueBindingRepo *repository.QueueBindingRepo,
	showProfileRepo *repository.ShowProfileRepo,
	engine *rotation.Engine,
) *Service {
	return &Service{
		playlistName:     playlistName,
		plexClient:       plexClient,
		serverRepo:       serverRepo,
		seriesRepo:       seriesRepo,
		episodeRepo:      episodeRepo,
		progressRepo:     progressRepo,
		policyRepo:       policyRepo,
		rotationRepo:     rotationRepo,
		bindingRepo:      bindingRepo,
		playlistRepo:     playlistRepo,
		queueBindingRepo: queueBindingRepo,
		showProfileRepo:  showProfileRepo,
		engine:           engine,
		playlistLocks:    make(map[string]*sync.Mutex),
	}
}

// --- Media Server ---

func (s *Service) ListServers(ctx context.Context) ([]repository.MediaServer, error) {
	servers, err := s.serverRepo.List(ctx)
	if err != nil {
		slog.Error("list servers failed", "error", err)
		return nil, err
	}
	if servers == nil {
		servers = make([]repository.MediaServer, 0)
	}
	// Mask tokens
	for i := range servers {
		if len(servers[i].Token) > 8 {
			servers[i].Token = servers[i].Token[:8] + "..."
		}
	}
	return servers, nil
}

func (s *Service) CreateServer(ctx context.Context, url, token string) (*repository.MediaServer, error) {
	id := uuid.New().String()
	server, err := s.serverRepo.Create(ctx, id, url, token, "Plex Server")
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	// Import series from the new server
	if err := s.importSeries(ctx, server); err != nil {
		slog.Warn("initial series import failed", "server_id", id, "error", err)
	}

	return server, nil
}

func (s *Service) TestServer(ctx context.Context, id string) error {
	server, err := s.serverRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	client := plex.NewClient(server.URL, server.Token, 30*time.Second)
	return client.TestConnection(ctx)
}

func (s *Service) importSeries(ctx context.Context, server *repository.MediaServer) error {
	client := plex.NewClient(server.URL, server.Token, 30*time.Second)

	libraries, err := client.ListLibraries(ctx)
	if err != nil {
		return fmt.Errorf("list libraries: %w", err)
	}

	for _, lib := range libraries {
		if lib.Type != "show" {
			continue
		}

		seriesList, err := client.ListSeries(ctx, lib.ID)
		if err != nil {
			slog.Warn("list series failed", "library_id", lib.ID, "error", err)
			continue
		}

		for _, ms := range seriesList {
			series := &repository.Series{
				ID:             uuid.New().String(),
				MediaServerID:  server.ID,
				ServerSeriesID: ms.ID,
				LibraryID:      lib.ID,
				Title:          ms.Title,
				Active:         false,
			}
			if err := s.seriesRepo.Upsert(ctx, series); err != nil {
				slog.Warn("upsert series failed", "title", ms.Title, "error", err)
				continue
			}
		}
	}

	return nil
}

// --- Series ---

func (s *Service) ListSeries(ctx context.Context) ([]repository.Series, error) {
	series, err := s.seriesRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	if series == nil {
		series = make([]repository.Series, 0)
	}
	return series, nil
}

func (s *Service) GetSeries(ctx context.Context, id string) (interface{}, error) {
	series, err := s.seriesRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	progress, err := s.progressRepo.GetBySeries(ctx, id)
	if err != nil {
		progress = nil
	}

	result := map[string]interface{}{
		"id":              series.ID,
		"title":           series.Title,
		"active":          series.Active,
		"media_server_id": series.MediaServerID,
		"library_id":      series.LibraryID,
		"created_at":      series.CreatedAt,
		"updated_at":      series.UpdatedAt,
	}
	if progress != nil {
		result["progress"] = progress
	}

	return result, nil
}

func (s *Service) SetActive(ctx context.Context, id string, active bool) error {
	if err := s.seriesRepo.SetActive(ctx, id, active); err != nil {
		return fmt.Errorf("set active: %w", err)
	}

	if active {
		// Initialize cursor for this series
		if err := s.initializeCursor(ctx, id); err != nil {
			slog.Warn("cursor initialization failed", "series_id", id, "error", err)
		}
	}

	return nil
}

func (s *Service) initializeCursor(ctx context.Context, seriesID string) error {
	episodes, err := s.episodeRepo.ListBySeries(ctx, seriesID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}

	if len(episodes) == 0 {
		// Import episodes first
		series, err := s.seriesRepo.GetByID(ctx, seriesID)
		if err != nil {
			return err
		}
		if err := s.importEpisodes(ctx, series); err != nil {
			return err
		}
		episodes, err = s.episodeRepo.ListBySeries(ctx, seriesID)
		if err != nil {
			return err
		}
	}

	if len(episodes) == 0 {
		return fmt.Errorf("no episodes found for series")
	}

	first := episodes[0]
	progress := &repository.SeriesProgress{
		SeriesID:      seriesID,
		NextEpisodeID: &first.ID,
		NextPosition:  &first.AbsoluteOrder,
	}
	return s.progressRepo.Upsert(ctx, progress)
}

func (s *Service) SyncSeries(ctx context.Context, id string) error {
	series, err := s.seriesRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.importEpisodes(ctx, series)
}

func (s *Service) ReconcileSeries(ctx context.Context, id string) error {
	series, err := s.seriesRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	progress, err := s.progressRepo.GetBySeries(ctx, id)
	if err != nil {
		return fmt.Errorf("get progress: %w", err)
	}

	if progress.NextEpisodeID == nil {
		return fmt.Errorf("series has no next episode")
	}

	episode, err := s.episodeRepo.GetByID(ctx, *progress.NextEpisodeID)
	if err != nil {
		return err
	}

	box := []string{episode.ServerEpisodeID}
	progressList, err := s.plexClient.GetEpisodeProgress(ctx, box)
	if err != nil {
		return fmt.Errorf("get episode progress: %w", err)
	}

	if len(progressList) > 0 && progressList[0].ViewCount > 0 && progressList[0].LastViewedAt > 0 {
		return s.advanceCursor(ctx, series.ID, *progress.NextEpisodeID)
	}

	return nil
}

func (s *Service) importEpisodes(ctx context.Context, series *repository.Series) error {
	server, err := s.serverRepo.GetByID(ctx, series.MediaServerID)
	if err != nil {
		return err
	}

	client := plex.NewClient(server.URL, server.Token, 30*time.Second)

	episodes, err := client.ListEpisodes(ctx, series.ServerSeriesID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}

	if len(episodes) == 0 {
		return fmt.Errorf("no episodes returned by Plex for series %s", series.ServerSeriesID)
	}

	imported := make([]*repository.Episode, 0, len(episodes))
	for i, ep := range episodes {
		absOrder := i + 1
		imported = append(imported, &repository.Episode{
			ID:              uuid.New().String(),
			SeriesID:        series.ID,
			ServerEpisodeID: ep.ID,
			SeasonNumber:    ep.SeasonNumber,
			EpisodeNumber:   ep.EpisodeNumber,
			AbsoluteOrder:   absOrder,
			Title:           ep.Title,
			Duration:        ep.Duration,
			Rating:          ep.Rating,
			AirDate:         ep.AirDate,
		})
	}

	if err := s.episodeRepo.UpsertAll(ctx, imported); err != nil {
		return fmt.Errorf("upsert episodes: %w", err)
	}

	if len(imported) == 0 {
		return fmt.Errorf("no episodes imported for series %s", series.ServerSeriesID)
	}

	return nil
}

// --- Cursor ---

func (s *Service) advanceCursor(ctx context.Context, seriesID string, watchedEpisodeID string) error {
	episodes, err := s.episodeRepo.ListBySeries(ctx, seriesID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}

	var nextEpisode *repository.Episode
	found := false
	for _, ep := range episodes {
		if ep.ID == watchedEpisodeID {
			found = true
			continue
		}
		if found && ep.ID != watchedEpisodeID {
			nextEpisode = &ep
			break
		}
	}

	if !found {
		return fmt.Errorf("watched episode not found in series")
	}

	var nextID *string
	var nextPos *int
	if nextEpisode != nil {
		nextID = &nextEpisode.ID
		nextPos = &nextEpisode.AbsoluteOrder
	}

	return s.progressRepo.Advance(ctx, seriesID, watchedEpisodeID, nextID, nextPos)
}

// --- Rotation Profiles ---

func (s *Service) ListProfiles(ctx context.Context) ([]repository.RotationProfile, error) {
	profiles, err := s.policyRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	if profiles == nil {
		profiles = make([]repository.RotationProfile, 0)
	}
	return profiles, nil
}

func (s *Service) CreateProfile(ctx context.Context, policy *rotation.Policy) (*repository.RotationProfile, error) {
	config, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}
	return s.policyRepo.Create(ctx, uuid.New().String(), "Rotation Profile", config)
}

func (s *Service) UpdateProfile(ctx context.Context, id string, policy *rotation.Policy) (*repository.RotationProfile, error) {
	config, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}
	return s.policyRepo.Update(ctx, id, "Rotation Profile", config)
}

func (s *Service) PreviewProfile(ctx context.Context, id string) (*rotation.RotationResult, error) {
	profile, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var policy rotation.Policy
	if err := json.Unmarshal(profile.Configuration, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}

	candidates, err := s.buildCandidates(ctx)
	if err != nil {
		return nil, err
	}

	seed, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return s.engine.Generate(candidates, &policy, seed.Int64())
}

// --- Rotation ---

func (s *Service) CurrentRotation(ctx context.Context) (interface{}, error) {
	rot, err := s.rotationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	items, err := s.rotationRepo.ListItems(ctx, rot.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"rotation": rot,
		"items":    items,
	}, nil
}

func (s *Service) GenerateRotation(ctx context.Context) (*rotation.RotationResult, error) {
	profile, err := s.policyRepo.GetFirstEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("no enabled profile: %w", err)
	}

	var policy rotation.Policy
	if err := json.Unmarshal(profile.Configuration, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}

	candidates, err := s.buildCandidates(ctx)
	if err != nil {
		return nil, err
	}

	seed, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	seedVal := seed.Int64()

	result, err := s.engine.Generate(candidates, &policy, seedVal)
	if err != nil {
		return nil, fmt.Errorf("generate rotation: %w", err)
	}

	// Persist the rotation
	rotID := uuid.New().String()
	_, err = s.rotationRepo.Create(ctx, rotID, profile.ID, "draft", seedVal, policy.Constraints.SessionBudgetMinutes)
	if err != nil {
		return nil, fmt.Errorf("persist rotation: %w", err)
	}

	for _, item := range result.Items {
		details, _ := json.Marshal(item.ScoreDetails)
		ri := &repository.RotationItem{
			ID:           uuid.New().String(),
			RotationID:   rotID,
			Position:     item.Position,
			SeriesID:     item.SeriesID,
			EpisodeID:    item.EpisodeID,
			SlotKind:     item.SlotKind,
			Score:        item.Score,
			ScoreDetails: details,
			Status:       "pending",
		}
		if err := s.rotationRepo.AddItem(ctx, ri); err != nil {
			return nil, fmt.Errorf("persist rotation item: %w", err)
		}
	}

	return result, nil
}

func (s *Service) PublishRotation(ctx context.Context, id string) error {
	rot, err := s.rotationRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	items, err := s.rotationRepo.ListItems(ctx, rot.ID)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		return fmt.Errorf("no items in rotation")
	}

	// Get server
	servers, err := s.serverRepo.List(ctx)
	if err != nil || len(servers) == 0 {
		return fmt.Errorf("no media servers configured")
	}

	server := &servers[0]
	client := plex.NewClient(server.URL, server.Token, 30*time.Second)

	// Collect episode IDs
	episodeIDs := make([]string, len(items))
	for i, item := range items {
		episode, err := s.episodeRepo.GetByID(ctx, item.EpisodeID)
		if err != nil {
			return fmt.Errorf("get episode %s: %w", item.EpisodeID, err)
		}
		episodeIDs[i] = episode.ServerEpisodeID
	}

	// Check for existing binding
	binding, err := s.bindingRepo.GetByProfile(ctx, rot.ProfileID)
	if err != nil {
		binding = nil
	}

	var playlistID *string
	if binding != nil {
		playlistID = binding.ServerPlaylistID
	}

	playlist, err := client.UpsertPlaylist(ctx, playlistID, s.playlistName, episodeIDs)
	if err != nil {
		return fmt.Errorf("publish playlist: %w", err)
	}

	// Save binding
	now := time.Now()
	bindingRecord := &repository.PlaylistBinding{
		ID:                uuid.New().String(),
		MediaServerID:     server.ID,
		RotationProfileID: rot.ProfileID,
		ServerPlaylistID:  &playlist.ID,
		PlaylistName:      s.playlistName,
		SynchronizedAt:    &now,
	}
	if err := s.bindingRepo.Upsert(ctx, bindingRecord); err != nil {
		slog.Warn("save playlist binding failed", "error", err)
	}

	// Mark rotation as published
	return s.rotationRepo.SetStatus(ctx, rot.ID, "published")
}

func (s *Service) PublishCurrentRotation(ctx context.Context) error {
	rot, err := s.rotationRepo.GetCurrent(ctx)
	if err != nil {
		return err
	}
	return s.PublishRotation(ctx, rot.ID)
}

func (s *Service) RerollRotation(ctx context.Context, id string) (*rotation.RotationResult, error) {
	rot, err := s.rotationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	profile, err := s.policyRepo.GetByID(ctx, rot.ProfileID)
	if err != nil {
		return nil, err
	}

	var policy rotation.Policy
	if err := json.Unmarshal(profile.Configuration, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}

	candidates, err := s.buildCandidates(ctx)
	if err != nil {
		return nil, err
	}

	seed, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	seedVal := seed.Int64()

	result, err := s.engine.Generate(candidates, &policy, seedVal)
	if err != nil {
		return nil, err
	}

	// Create new rotation
	rotID := uuid.New().String()
	_, err = s.rotationRepo.Create(ctx, rotID, rot.ProfileID, "draft", seedVal, policy.Constraints.SessionBudgetMinutes)
	if err != nil {
		return nil, fmt.Errorf("persist rotation: %w", err)
	}

	for _, item := range result.Items {
		details, _ := json.Marshal(item.ScoreDetails)
		ri := &repository.RotationItem{
			ID:           uuid.New().String(),
			RotationID:   rotID,
			Position:     item.Position,
			SeriesID:     item.SeriesID,
			EpisodeID:    item.EpisodeID,
			SlotKind:     item.SlotKind,
			Score:        item.Score,
			ScoreDetails: details,
			Status:       "pending",
		}
		if err := s.rotationRepo.AddItem(ctx, ri); err != nil {
			return nil, fmt.Errorf("persist rotation item: %w", err)
		}
	}

	// Mark old rotation as cancelled
	if err := s.rotationRepo.SetStatus(ctx, rot.ID, "cancelled"); err != nil {
		slog.Warn("cancel old rotation failed", "rotation_id", rot.ID, "error", err)
	}

	return result, nil
}

func (s *Service) SyncRotation(ctx context.Context) error {
	rot, err := s.rotationRepo.GetCurrent(ctx)
	if err != nil {
		return nil
	}

	items, err := s.rotationRepo.ListItems(ctx, rot.ID)
	if err != nil {
		return err
	}

	// Get active episodes to check progress
	episodeIDs := make([]string, 0, len(items))
	itemMap := make(map[string]*repository.RotationItem)
	for _, item := range items {
		episode, err := s.episodeRepo.GetByID(ctx, item.EpisodeID)
		if err != nil {
			continue
		}
		episodeIDs = append(episodeIDs, episode.ServerEpisodeID)
		itemMap[episode.ServerEpisodeID] = &item
	}

	if len(episodeIDs) == 0 {
		return nil
	}

	progressList, err := s.plexClient.GetEpisodeProgress(ctx, episodeIDs)
	if err != nil {
		return fmt.Errorf("get progress: %w", err)
	}

	allWatched := true
	for _, p := range progressList {
		item, ok := itemMap[p.EpisodeID]
		if !ok {
			continue
		}

		newlyViewed := newlyViewedAfter(p, rot.GeneratedAt)
		if newlyViewed && item.Status != "watched" {
			if err := s.rotationRepo.UpdateItemStatus(ctx, item.ID, "watched"); err != nil {
				slog.Warn("update item status failed", "item_id", item.ID, "error", err)
				continue
			}

			// Advance cursor
			if err := s.advanceCursor(ctx, item.SeriesID, item.EpisodeID); err != nil {
				slog.Warn("advance cursor failed", "series_id", item.SeriesID, "error", err)
			}
		}

		if newlyViewed {
			allWatched = allWatched && true
		} else {
			allWatched = false
		}
	}

	if allWatched && len(items) > 0 {
		if err := s.rotationRepo.SetStatus(ctx, rot.ID, "completed"); err != nil {
			slog.Warn("complete rotation failed", "error", err)
		}
	}

	return nil
}

func (s *Service) SyncAll(ctx context.Context) error {
	servers, err := s.serverRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		client := plex.NewClient(server.URL, server.Token, 30*time.Second)

		// Import series from libraries
		libraries, err := client.ListLibraries(ctx)
		if err != nil {
			slog.Warn("list libraries failed", "server_id", server.ID, "error", err)
			continue
		}

		for _, lib := range libraries {
			if lib.Type != "show" {
				continue
			}

			seriesList, err := client.ListSeries(ctx, lib.ID)
			if err != nil {
				slog.Warn("list series failed", "library_id", lib.ID, "error", err)
				continue
			}

			for _, ms := range seriesList {
				series := &repository.Series{
					ID:             uuid.New().String(),
					MediaServerID:  server.ID,
					ServerSeriesID: ms.ID,
					LibraryID:      lib.ID,
					Title:          ms.Title,
					Active:         false,
				}
				if err := s.seriesRepo.Upsert(ctx, series); err != nil {
					slog.Warn("upsert series failed", "title", ms.Title, "error", err)
					continue
				}

				// Import episodes
				if err := s.importEpisodes(ctx, series); err != nil {
					slog.Warn("import episodes failed", "series_id", series.ID, "error", err)
					continue
				}

				// Initialize cursor if active
				if series.Active {
					if err := s.initializeCursor(ctx, series.ID); err != nil {
						slog.Warn("initialize cursor failed", "series_id", series.ID, "error", err)
					}
				}
			}
		}
	}

	// Sync current rotation
	if err := s.SyncRotation(ctx); err != nil {
		slog.Warn("sync rotation failed", "error", err)
	}

	return nil
}

// --- Candidate building ---

func (s *Service) buildCandidates(ctx context.Context) ([]rotation.Candidate, error) {
	activeSeries, err := s.seriesRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active series: %w", err)
	}

	var candidates []rotation.Candidate

	for _, series := range activeSeries {
		progress, err := s.progressRepo.GetBySeries(ctx, series.ID)
		if err != nil || progress.NextEpisodeID == nil {
			continue
		}

		episode, err := s.episodeRepo.GetByID(ctx, *progress.NextEpisodeID)
		if err != nil {
			continue
		}
		if episode.Unavailable {
			continue
		}

		// Build window ratings
		episodes, err := s.episodeRepo.ListBySeries(ctx, series.ID)
		if err != nil {
			continue
		}

		var windowRatings []float64
		startIdx := episode.AbsoluteOrder - 1
		for i := startIdx; i < len(episodes) && i < startIdx+5; i++ {
			windowRatings = append(windowRatings, episodes[i].Rating)
		}

		candidates = append(candidates, rotation.Candidate{
			SeriesID:      series.ID,
			EpisodeID:     episode.ID,
			SeriesTitle:   series.Title,
			EpisodeTitle:  episode.Title,
			SeasonNumber:  episode.SeasonNumber,
			EpisodeNumber: episode.EpisodeNumber,
			Duration:      episode.Duration,
			EpisodeRating: episode.Rating,
			WindowRatings: windowRatings,
		})
	}

	return candidates, nil
}

// --- Playlists ---

type fillCandidate struct {
	seriesID   string
	psID       string
	mode       string
	episodeID  string
	rating     float64
	lastSeenAt *time.Time
}

type PlaylistResponse struct {
	ID                             string                    `json:"id"`
	MediaServerID                  string                    `json:"media_server_id"`
	Name                           string                    `json:"name"`
	PlexPlaylistName               string                    `json:"plex_playlist_name"`
	QueueTargetCount               int                       `json:"queue_target_count"`
	CycleCursor                    int                       `json:"cycle_cursor"`
	Enabled                        bool                      `json:"enabled"`
	CreatedAt                      time.Time                 `json:"created_at"`
	UpdatedAt                      time.Time                 `json:"updated_at"`
	Series                         []PlaylistSeriesResponse  `json:"series,omitempty"`
	Slots                          []repository.PlaylistSlot `json:"slots,omitempty"`
	QueueItems                     []PlaylistQueueResponse   `json:"queue_items,omitempty"`
	QueuePending                   int                       `json:"queue_pending_count"`
	RemainingSerialDurationSeconds int                       `json:"remaining_serial_duration_seconds"`
	Revision                       string                    `json:"revision"`
}

type PlaylistSeriesResponse struct {
	ID                    string     `json:"id"`
	SeriesID              string     `json:"series_id"`
	Title                 string     `json:"title"`
	Mode                  string     `json:"mode"`
	RandomEpisodeCooldown int        `json:"random_episode_cooldown"`
	NextPosition          *int       `json:"next_position,omitempty"`
	NextEpisodeID         *string    `json:"next_episode_id,omitempty"`
	NextEpisodeTitle      string     `json:"next_episode_title,omitempty"`
	NextSeasonNumber      int        `json:"next_season_number,omitempty"`
	NextEpisodeNumber     int        `json:"next_episode_number,omitempty"`
	TotalEpisodes         int        `json:"total_episodes"`
	WatchedEpisodes       int        `json:"watched_episodes"`
	ProgressPct           float64    `json:"progress_pct"`
	ShowProfileID         *string    `json:"show_profile_id,omitempty"`
	ShowProfileName       string     `json:"show_profile_name,omitempty"`
	EligibleEpisodes      int        `json:"eligible_episodes"`
	LastSeenAt            *time.Time `json:"last_seen_at,omitempty"`
}

type PlaylistQueueResponse struct {
	ID              string   `json:"id"`
	Position        int      `json:"position"`
	CycleIndex      int      `json:"cycle_index"`
	SlotPosition    int      `json:"slot_position"`
	SlotType        string   `json:"slot_type"`
	SeriesID        string   `json:"series_id"`
	SeriesTitle     string   `json:"series_title"`
	EpisodeID       string   `json:"episode_id"`
	ServerEpisodeID string   `json:"server_episode_id"`
	EpisodeTitle    string   `json:"episode_title"`
	SeasonNumber    int      `json:"season_number"`
	EpisodeNumber   int      `json:"episode_number"`
	EpisodeRating   *float64 `json:"episode_rating"`
	Score           *float64 `json:"score"`
	Status          string   `json:"status"`
}

type PlexPlaylistItemResponse struct {
	QueueItemID     string `json:"queue_item_id"`
	ServerEpisodeID string `json:"server_episode_id"`
	SeriesTitle     string `json:"series_title"`
	EpisodeTitle    string `json:"episode_title"`
	SeasonNumber    int    `json:"season_number"`
	EpisodeNumber   int    `json:"episode_number"`
}

type PlexPlaylistResponse struct {
	Items        []PlexPlaylistItemResponse `json:"items"`
	BaseRevision string                     `json:"base_revision"`
}

type StalePlaylistEditError struct{ Expected, Actual string }

func (e *StalePlaylistEditError) Error() string {
	return fmt.Sprintf("playlist edit is stale: base_revision=%s current_revision=%s", e.Expected, e.Actual)
}

func (s *Service) playlistLock(id string) *sync.Mutex {
	s.playlistLocksMu.Lock()
	defer s.playlistLocksMu.Unlock()
	if lock := s.playlistLocks[id]; lock != nil {
		return lock
	}
	lock := &sync.Mutex{}
	s.playlistLocks[id] = lock
	return lock
}

func queueRevision(items []repository.PlaylistQueueItem) string {
	var b strings.Builder
	for _, item := range items {
		fmt.Fprintf(&b, "%s:%d:%s:%s|", item.ID, item.Position, item.EpisodeID, item.Status)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(b.String())))
}

func (s *Service) ListPlaylists(ctx context.Context) ([]PlaylistResponse, error) {
	playlists, err := s.playlistRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]PlaylistResponse, 0)
	for _, p := range playlists {
		pending, _ := s.playlistRepo.CountPendingQueueItems(ctx, p.ID)
		res = append(res, PlaylistResponse{
			ID:               p.ID,
			MediaServerID:    p.MediaServerID,
			Name:             p.Name,
			PlexPlaylistName: p.PlexPlaylistName,
			QueueTargetCount: p.QueueTargetCount,
			CycleCursor:      p.CycleCursor,
			Enabled:          p.Enabled,
			CreatedAt:        p.CreatedAt,
			UpdatedAt:        p.UpdatedAt,
			QueuePending:     pending,
		})
	}
	return res, nil
}

func (s *Service) CreatePlaylist(ctx context.Context, mediaServerID, name, plexName string, targetCount int) (*PlaylistResponse, error) {
	id := uuid.New().String()
	playlist := &repository.Playlist{
		ID:               id,
		MediaServerID:    mediaServerID,
		Name:             name,
		PlexPlaylistName: plexName,
		QueueTargetCount: targetCount,
		CycleCursor:      0,
		Enabled:          true,
	}
	if _, err := s.playlistRepo.Create(ctx, playlist); err != nil {
		return nil, err
	}
	if err := s.playlistRepo.EnsureDefaultSlot(ctx, id); err != nil {
		slog.Warn("default slot creation failed", "playlist_id", id, "error", err)
	}
	return &PlaylistResponse{
		ID:               id,
		MediaServerID:    mediaServerID,
		Name:             name,
		PlexPlaylistName: plexName,
		QueueTargetCount: targetCount,
		Enabled:          true,
	}, nil
}

func (s *Service) GetPlaylist(ctx context.Context, id string) (*PlaylistResponse, error) {
	p, err := s.playlistRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	members, err := s.playlistRepo.ListSeries(ctx, id)
	if err != nil {
		return nil, err
	}

	slots, err := s.playlistRepo.ListSlots(ctx, id)
	if err != nil {
		return nil, err
	}

	queueItems, err := s.playlistRepo.ListQueueItems(ctx, id)
	if err != nil {
		return nil, err
	}

	pending, _ := s.playlistRepo.CountPendingQueueItems(ctx, id)

	seriesResp := make([]PlaylistSeriesResponse, 0, len(members))
	remainingSerialDuration := 0
	for _, m := range members {
		ser, err := s.seriesRepo.GetByID(ctx, m.SeriesID)
		if err != nil {
			continue
		}
		sr := PlaylistSeriesResponse{
			ID:                    m.ID,
			SeriesID:              m.SeriesID,
			Title:                 ser.Title,
			Mode:                  m.Mode,
			RandomEpisodeCooldown: m.RandomEpisodeCooldown,
			ShowProfileID:         m.ShowProfileID,
			LastSeenAt:            m.LastSeenAt,
		}
		if m.ShowProfileID != nil {
			if profile, err := s.showProfileRepo.GetByID(ctx, *m.ShowProfileID); err == nil {
				sr.ShowProfileName = profile.Name
			}
		}

		total, _ := s.episodeRepo.CountBySeries(ctx, m.SeriesID)
		sr.TotalEpisodes = total
		episodes, episodesErr := s.episodeRepo.ListBySeries(ctx, m.SeriesID)
		rules, rulesErr := s.profileRules(ctx, m.ShowProfileID)
		if episodesErr == nil && rulesErr == nil {
			sr.EligibleEpisodes = len(filterAllowedEpisodes(episodes, rules))
		}

		if m.Mode == "serial" {
			progress, _ := s.playlistRepo.GetProgress(ctx, m.ID)
			if progress != nil {
				sr.NextPosition = progress.NextPosition
				sr.NextEpisodeID = progress.NextEpisodeID
				if progress.NextEpisodeID != nil {
					if ep, err := s.episodeRepo.GetByID(ctx, *progress.NextEpisodeID); err == nil {
						sr.NextEpisodeTitle = ep.Title
						sr.NextSeasonNumber = ep.SeasonNumber
						sr.NextEpisodeNumber = ep.EpisodeNumber
					}
				}
				// Setting a serial cursor to episode N explicitly means episodes
				// before N are considered watched, even without Plex watch history.
				if progress.NextPosition != nil {
					sr.WatchedEpisodes = max(0, *progress.NextPosition-1)
				} else if progress.LastWatchedEpisodeID != nil {
					sr.WatchedEpisodes = total
				}
			}
			if episodesErr == nil && rulesErr == nil && (progress == nil || progress.NextPosition != nil) {
				var nextPosition int
				if progress != nil {
					nextPosition = *progress.NextPosition
				}
				remainingSerialDuration += remainingDuration(episodes, nextPosition, rules)
			}
		} else {
			historyCount, _ := s.playlistRepo.GetHistoryCount(ctx, m.ID)
			sr.WatchedEpisodes = historyCount
		}

		if total > 0 {
			if sr.WatchedEpisodes > total {
				sr.WatchedEpisodes = total
			}
			sr.ProgressPct = float64(sr.WatchedEpisodes) / float64(total) * 100.0
		}

		seriesResp = append(seriesResp, sr)
	}

	queueResp := make([]PlaylistQueueResponse, 0, len(queueItems))
	for _, qi := range queueItems {
		q := PlaylistQueueResponse{
			ID:           qi.ID,
			Position:     qi.Position,
			CycleIndex:   qi.CycleIndex,
			SlotPosition: qi.SlotPosition,
			SlotType:     qi.SlotType,
			SeriesID:     qi.SeriesID,
			EpisodeID:    qi.EpisodeID,
			Score:        qi.Score,
			Status:       qi.Status,
		}
		ep, _ := s.episodeRepo.GetByID(ctx, qi.EpisodeID)
		if ep != nil {
			q.ServerEpisodeID = ep.ServerEpisodeID
			q.EpisodeTitle = ep.Title
			q.SeasonNumber = ep.SeasonNumber
			q.EpisodeNumber = ep.EpisodeNumber
			if ep.Rating > 0 {
				rating := ep.Rating
				q.EpisodeRating = &rating
			}
		}
		series, _ := s.seriesRepo.GetByID(ctx, qi.SeriesID)
		if series != nil {
			q.SeriesTitle = series.Title
		}
		queueResp = append(queueResp, q)
	}

	return &PlaylistResponse{
		ID:                             p.ID,
		MediaServerID:                  p.MediaServerID,
		Name:                           p.Name,
		PlexPlaylistName:               p.PlexPlaylistName,
		QueueTargetCount:               p.QueueTargetCount,
		CycleCursor:                    p.CycleCursor,
		Enabled:                        p.Enabled,
		CreatedAt:                      p.CreatedAt,
		UpdatedAt:                      p.UpdatedAt,
		Series:                         seriesResp,
		Slots:                          slots,
		QueueItems:                     queueResp,
		QueuePending:                   pending,
		RemainingSerialDurationSeconds: remainingSerialDuration,
		Revision:                       queueRevision(queueItems),
	}, nil
}

func remainingDuration(episodes []repository.Episode, nextPosition int, rules ShowProfileRules) int {
	total := 0
	for _, episode := range episodes {
		if episode.AbsoluteOrder >= nextPosition && !episode.Unavailable && rules.Allows(episode) {
			total += episode.Duration
		}
	}
	return total
}

func (s *Service) UpdatePlaylist(ctx context.Context, id, name, plexName string, targetCount int, enabled bool) error {
	lock := s.playlistLock(id)
	lock.Lock()
	defer lock.Unlock()
	p, err := s.playlistRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	p.Name = name
	p.PlexPlaylistName = plexName
	p.QueueTargetCount = targetCount
	p.Enabled = enabled
	return s.playlistRepo.Update(ctx, p)
}

func (s *Service) DeletePlaylist(ctx context.Context, id string) error {
	return s.playlistRepo.Delete(ctx, id)
}

func (s *Service) SetPlaylistSeries(ctx context.Context, playlistID string, series []struct {
	SeriesID              string  `json:"series_id"`
	Mode                  string  `json:"mode"`
	RandomEpisodeCooldown *int    `json:"random_episode_cooldown"`
	ShowProfileID         *string `json:"show_profile_id"`
}) error {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	playlist, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("get playlist: %w", err)
	}

	// Determine which series are new so we only import episodes once
	current, err := s.playlistRepo.ListSeries(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("list current: %w", err)
	}
	currentSet := make(map[string]repository.PlaylistSeries)
	for _, m := range current {
		currentSet[m.SeriesID] = m
	}

	input := make([]repository.PlaylistSeriesInput, 0, len(series))
	for _, sr := range series {
		if sr.Mode != "serial" && sr.Mode != "non_serial" {
			return fmt.Errorf("invalid mode: %s", sr.Mode)
		}
		ser, err := s.seriesRepo.GetByID(ctx, sr.SeriesID)
		if err != nil {
			return fmt.Errorf("series %s: %w", sr.SeriesID, err)
		}
		if ser.MediaServerID != playlist.MediaServerID {
			return fmt.Errorf("series %s does not belong to the playlist's media server", sr.SeriesID)
		}

		// New members and older members without local episodes both require import.
		episodeCount, err := s.episodeRepo.CountBySeries(ctx, sr.SeriesID)
		if err != nil {
			return fmt.Errorf("count episodes for %s: %w", sr.SeriesID, err)
		}
		if _, exists := currentSet[sr.SeriesID]; !exists || episodeCount == 0 {
			if err := s.importEpisodes(ctx, ser); err != nil {
				return fmt.Errorf("import episodes for %s: %w", sr.SeriesID, err)
			}
		}

		profileID := sr.ShowProfileID
		cooldown := 10
		if sr.RandomEpisodeCooldown != nil {
			cooldown = *sr.RandomEpisodeCooldown
		} else if existing, ok := currentSet[sr.SeriesID]; ok {
			cooldown = existing.RandomEpisodeCooldown
		}
		if cooldown < 0 {
			return fmt.Errorf("random episode cooldown must be non-negative")
		}
		if profileID != nil {
			if _, err := s.profileForSeries(ctx, sr.SeriesID, *profileID); err != nil {
				return err
			}
		} else if existing, ok := currentSet[sr.SeriesID]; ok {
			profileID = existing.ShowProfileID
		} else {
			profile, err := s.showProfileRepo.EnsureDefaultForSeries(ctx, sr.SeriesID, uuid.NewString())
			if err != nil {
				return err
			}
			profileID = &profile.ID
		}
		input = append(input, repository.PlaylistSeriesInput{
			SeriesID:              sr.SeriesID,
			Mode:                  sr.Mode,
			RandomEpisodeCooldown: cooldown,
			ShowProfileID:         profileID,
		})
	}

	if err := s.playlistRepo.SetPlaylistSeries(ctx, playlistID, input); err != nil {
		return fmt.Errorf("set series: %w", err)
	}

	// Initialize cursors for new serial members
	members, err := s.playlistRepo.ListSeries(ctx, playlistID)
	if err != nil {
		return nil
	}
	for _, m := range members {
		if m.Mode != "serial" {
			continue
		}
		progress, err := s.playlistRepo.GetProgress(ctx, m.ID)
		if err == nil && progress != nil {
			continue
		}
		if _, _, err := s.initPlaylistCursor(ctx, m); err != nil {
			return fmt.Errorf("init cursor for %s: %w", m.SeriesID, err)
		}
	}

	return nil
}

func (s *Service) SetPlaylistSlots(ctx context.Context, playlistID string, slotTypes []string) error {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	slots := make([]repository.PlaylistSlot, 0, len(slotTypes))
	for i, st := range slotTypes {
		slots = append(slots, repository.PlaylistSlot{
			ID:         uuid.New().String(),
			PlaylistID: playlistID,
			Position:   i,
			SlotType:   st,
		})
	}
	return s.playlistRepo.SetSlots(ctx, playlistID, slots)
}

func (s *Service) FillPlaylist(ctx context.Context, playlistID string) (int, error) {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	return s.fillPlaylist(ctx, playlistID)
}

func (s *Service) fillPlaylist(ctx context.Context, playlistID string) (int, error) {
	playlist, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return 0, err
	}

	if err := s.playlistRepo.EnsureDefaultSlot(ctx, playlistID); err != nil {
		slog.Warn("ensure default slot failed", "error", err)
	}

	slots, err := s.playlistRepo.ListSlots(ctx, playlistID)
	if err != nil {
		return 0, err
	}
	if len(slots) == 0 {
		return 0, fmt.Errorf("no slots configured")
	}

	members, err := s.playlistRepo.ListSeries(ctx, playlistID)
	if err != nil {
		return 0, err
	}
	if len(members) == 0 {
		return 0, fmt.Errorf("no series configured")
	}

	// Import episodes from Plex for all attached series so candidates are available
	for _, m := range members {
		ser, err := s.seriesRepo.GetByID(ctx, m.SeriesID)
		if err != nil {
			continue
		}
		if err := s.importEpisodes(ctx, ser); err != nil {
			slog.Warn("import episodes for fill", "series_id", m.SeriesID, "error", err)
		}
	}

	// A complete catalog refresh can confirm an episode is unavailable. Remove
	// only those confirmed stale queue references; Plex omissions alone never
	// reach this path.
	queuedBefore, _ := s.playlistRepo.ListPendingQueueItems(ctx, playlistID)
	for _, queued := range queuedBefore {
		if ep, err := s.episodeRepo.GetByID(ctx, queued.EpisodeID); err == nil && ep.Unavailable {
			if err := s.playlistRepo.UpdateItemStatus(ctx, queued.ID, "skipped"); err != nil {
				return 0, err
			}
		}
	}
	pending, err := s.playlistRepo.CountPendingQueueItems(ctx, playlistID)
	if err != nil {
		return 0, err
	}

	need := playlist.QueueTargetCount - pending
	if need <= 0 {
		return 0, nil
	}

	maxPos, err := s.playlistRepo.MaxQueuePosition(ctx, playlistID)
	if err != nil {
		return 0, err
	}

	// Build already-queued episode set to avoid duplicates
	existingItems, _ := s.playlistRepo.ListQueueItems(ctx, playlistID)
	queuedEpisodes := make(map[string]bool)
	for _, item := range existingItems {
		if item.Status == "pending" || item.Status == "pushed" || item.Status == "watching" {
			queuedEpisodes[item.EpisodeID] = true
		}
	}
	membersByID := make(map[string]repository.PlaylistSeries, len(members))
	for _, member := range members {
		membersByID[member.ID] = member
	}

	inserted := 0
	consumedSlots := 0
	for inserted < need {
		// A cycle is one pass over every configured slot. The queue may have
		// more candidates after a selection, so stopping on an attempt count can
		// incorrectly leave a target-sized queue short.
		candidates := make([]fillCandidate, 0, len(members))
		for _, m := range members {
			episodeID, rating, err := s.getPlaylistEpisode(ctx, m, playlistID, queuedEpisodes)
			if err != nil || episodeID == "" {
				continue
			}
			candidates = append(candidates, fillCandidate{
				seriesID: m.SeriesID, psID: m.ID, mode: m.Mode,
				episodeID: episodeID, rating: rating, lastSeenAt: m.LastSeenAt,
			})
		}

		insertedThisCycle := 0
		for slotOffset := 0; slotOffset < len(slots) && inserted < need; slotOffset++ {
			globalSlotPos := playlist.CycleCursor + consumedSlots
			cycleIndex := globalSlotPos / len(slots)
			slotPosition := globalSlotPos % len(slots)
			slot := slots[slotPosition]
			consumedSlots++

			selected, ok := selectFillCandidate(candidates, slot.SlotType, globalSlotPos, len(members))
			if !ok {
				continue
			}

			pos := maxPos + inserted + 1
			score := selected.rating
			item := &repository.PlaylistQueueItem{
				ID: uuid.New().String(), PlaylistID: playlistID,
				CycleIndex: cycleIndex, SlotPosition: slotPosition, SlotType: slot.SlotType,
				SeriesID: selected.seriesID, EpisodeID: selected.episodeID,
				Position: pos, Score: &score, Status: "pending",
			}
			if selected.mode == "serial" {
				item.PlaylistSeriesID = &selected.psID
			}
			if err := s.playlistRepo.AddQueueItem(ctx, item); err != nil {
				return 0, err
			}

			queuedEpisodes[selected.episodeID] = true
			for i, candidate := range candidates {
				if candidate.seriesID != selected.seriesID {
					continue
				}
				episodeID, rating, err := s.getPlaylistEpisode(ctx, membersByID[candidate.psID], playlistID, queuedEpisodes)
				if err != nil || episodeID == "" {
					candidates = append(candidates[:i], candidates[i+1:]...)
				} else {
					candidates[i].episodeID = episodeID
					candidates[i].rating = rating
				}
				break
			}
			inserted++
			insertedThisCycle++
		}

		if insertedThisCycle == 0 {
			break
		}
	}

	if consumedSlots > 0 {
		if err := s.playlistRepo.IncrementCursor(ctx, playlistID, consumedSlots); err != nil {
			return 0, err
		}
	}
	active, countErr := s.playlistRepo.CountPendingQueueItems(ctx, playlistID)
	if countErr != nil {
		return inserted, countErr
	}
	slog.Info("playlist fill complete", "playlist_id", playlistID, "target", playlist.QueueTargetCount, "active", active, "inserted", inserted, "member_count", len(members), "slot_count", len(slots))
	if active < playlist.QueueTargetCount {
		return inserted, fmt.Errorf("queue shortfall: target=%d active=%d eligible episodes could not fill the queue", playlist.QueueTargetCount, active)
	}

	return inserted, nil
}

func (s *Service) getPlaylistEpisode(ctx context.Context, member repository.PlaylistSeries, playlistID string, queued map[string]bool) (string, float64, error) {
	rules, err := s.profileRules(ctx, member.ShowProfileID)
	if err != nil {
		return "", 0, err
	}
	if member.Mode == "serial" {
		progress, err := s.playlistRepo.GetProgress(ctx, member.ID)
		if err != nil {
			return "", 0, err
		}
		if progress == nil {
			return s.initPlaylistCursor(ctx, member)
		}
		if progress.NextEpisodeID == nil {
			return "", 0, nil
		}
		episodes, err := s.episodeRepo.ListBySeries(ctx, member.SeriesID)
		if err != nil {
			return "", 0, nil
		}

		history, err := s.playlistRepo.HistoryEpisodeIDs(ctx, member.ID)
		if err != nil {
			return "", 0, err
		}
		if ep, ok := firstAllowedUnqueuedEpisodeAtCursorWithHistory(episodes, *progress.NextEpisodeID, progress.NextPosition, queued, history, rules); ok {
			return ep.ID, ep.Rating, nil
		}
		return "", 0, nil
	}

	// Non-serial: exclude only the most recently played episodes, then return
	// them to the pool as later episodes are watched.
	episodes, err := s.episodeRepo.ListBySeries(ctx, member.SeriesID)
	if err != nil {
		return "", 0, nil
	}
	if len(episodes) == 0 {
		return "", 0, nil
	}

	cooldown := effectiveRandomEpisodeCooldown(episodes, rules, queued, member.RandomEpisodeCooldown)
	history, err := s.playlistRepo.RecentHistoryEpisodeIDs(ctx, member.ID, cooldown)
	if err != nil {
		return "", 0, nil
	}

	eligible := eligibleRandomEpisodes(episodes, rules, history, queued)

	if len(eligible) == 0 {
		return "", 0, nil
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(eligible))))
	if err != nil {
		return "", 0, err
	}
	selected := eligible[n.Int64()]
	return selected.ID, selected.Rating, nil
}

func eligibleRandomEpisodes(episodes []repository.Episode, rules ShowProfileRules, recentHistory, queued map[string]bool) []repository.Episode {
	eligible := make([]repository.Episode, 0, len(episodes))
	for _, ep := range episodes {
		if ep.Unavailable {
			continue
		}
		if rules.Allows(ep) && !recentHistory[ep.ID] && !queued[ep.ID] {
			eligible = append(eligible, ep)
		}
	}
	return eligible
}

func effectiveRandomEpisodeCooldown(episodes []repository.Episode, rules ShowProfileRules, queued map[string]bool, cooldown int) int {
	available := 0
	for _, ep := range episodes {
		if ep.Unavailable {
			continue
		}
		if rules.Allows(ep) && !queued[ep.ID] {
			available++
		}
	}
	return min(cooldown, max(available-1, 0))
}

func firstUnqueuedEpisodeAtCursor(episodes []repository.Episode, nextEpisodeID string, nextPosition *int, queued map[string]bool) (repository.Episode, bool) {
	return firstAllowedUnqueuedEpisodeAtCursorWithHistory(episodes, nextEpisodeID, nextPosition, queued, nil, ShowProfileRules{DefaultAllow: true})
}

func firstAllowedUnqueuedEpisodeAtCursor(episodes []repository.Episode, nextEpisodeID string, nextPosition *int, queued map[string]bool, rules ShowProfileRules) (repository.Episode, bool) {
	return firstAllowedUnqueuedEpisodeAtCursorWithHistory(episodes, nextEpisodeID, nextPosition, queued, nil, rules)
}

func firstAllowedUnqueuedEpisodeAtCursorWithHistory(episodes []repository.Episode, nextEpisodeID string, nextPosition *int, queued, history map[string]bool, rules ShowProfileRules) (repository.Episode, bool) {
	atCursor := nextPosition != nil
	for _, ep := range episodes {
		if !atCursor {
			if ep.ID != nextEpisodeID {
				continue
			}
			atCursor = true
		}
		if nextPosition != nil && ep.AbsoluteOrder < *nextPosition {
			continue
		}
		if queued[ep.ID] || history[ep.ID] {
			continue
		}
		if ep.Unavailable {
			continue
		}
		if !rules.Allows(ep) {
			continue
		}
		return ep, true
	}
	return repository.Episode{}, false
}

func (s *Service) initPlaylistCursor(ctx context.Context, member repository.PlaylistSeries) (string, float64, error) {
	episodes, err := s.episodeRepo.ListBySeries(ctx, member.SeriesID)
	if err != nil {
		return "", 0, err
	}
	rules, err := s.profileRules(ctx, member.ShowProfileID)
	if err != nil {
		return "", 0, err
	}
	episodes = filterAllowedEpisodes(episodes, rules)
	if len(episodes) == 0 {
		return "", 0, fmt.Errorf("no episodes found")
	}

	first := episodes[0]
	progress := &repository.PlaylistProgress{
		ID:               uuid.New().String(),
		PlaylistSeriesID: member.ID,
		NextEpisodeID:    &first.ID,
		NextPosition:     &first.AbsoluteOrder,
	}
	if err := s.playlistRepo.UpsertProgress(ctx, progress); err != nil {
		return "", 0, err
	}

	return first.ID, first.Rating, nil
}

func (s *Service) advancePlaylistCursor(ctx context.Context, playlistSeriesID, watchedEpisodeID string) error {
	progress, err := s.playlistRepo.GetProgress(ctx, playlistSeriesID)
	if err != nil || progress == nil {
		return err
	}

	if progress.NextEpisodeID == nil || *progress.NextEpisodeID != watchedEpisodeID {
		return fmt.Errorf("episode mismatch for cursor advance")
	}

	member, err := s.playlistRepo.GetPlaylistSeriesByID(ctx, playlistSeriesID)
	if err != nil {
		return fmt.Errorf("get playlist series: %w", err)
	}

	episodes, err := s.episodeRepo.ListBySeries(ctx, member.SeriesID)
	if err != nil {
		return err
	}
	rules, err := s.profileRules(ctx, member.ShowProfileID)
	if err != nil {
		return err
	}
	history, err := s.playlistRepo.HistoryEpisodeIDs(ctx, playlistSeriesID)
	if err != nil {
		return err
	}

	var nextEpisode *repository.Episode
	advance := false
	for _, ep := range episodes {
		if ep.ID == watchedEpisodeID {
			advance = true
			continue
		}
		if advance && rules.Allows(ep) {
			if history[ep.ID] {
				continue
			}
			nextEpisode = &ep
			break
		}
	}

	var nextID *string
	var nextPos *int
	if nextEpisode != nil {
		nextID = &nextEpisode.ID
		nextPos = &nextEpisode.AbsoluteOrder
	}

	return s.playlistRepo.AdvanceProgress(ctx, playlistSeriesID, watchedEpisodeID, nextID, nextPos)
}

func (s *Service) ListPlaylistSeriesEpisodes(ctx context.Context, playlistID, seriesID string) ([]repository.Episode, error) {
	members, err := s.playlistRepo.ListSeries(ctx, playlistID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, m := range members {
		if m.SeriesID == seriesID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("series not in playlist")
	}
	return s.episodeRepo.ListBySeries(ctx, seriesID)
}

func (s *Service) SetPlaylistNextEpisode(ctx context.Context, playlistID, seriesID, episodeID string) error {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	ep, err := s.episodeRepo.GetByID(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("episode not found: %w", err)
	}
	if ep.SeriesID != seriesID {
		return fmt.Errorf("episode does not belong to series")
	}

	members, err := s.playlistRepo.ListSeries(ctx, playlistID)
	if err != nil {
		return err
	}
	var member *repository.PlaylistSeries
	for _, m := range members {
		if m.SeriesID == seriesID {
			if m.Mode != "serial" {
				return fmt.Errorf("series mode is not serial")
			}
			member = &m
			break
		}
	}
	if member == nil {
		return fmt.Errorf("series not in playlist")
	}
	rules, err := s.profileRules(ctx, member.ShowProfileID)
	if err != nil {
		return err
	}
	if !rules.Allows(*ep) {
		return fmt.Errorf("episode is excluded by this show's profile")
	}

	progress := &repository.PlaylistProgress{
		ID:               uuid.New().String(),
		PlaylistSeriesID: member.ID,
		NextEpisodeID:    &episodeID,
		NextPosition:     &ep.AbsoluteOrder,
	}
	if err := s.playlistRepo.UpsertProgress(ctx, progress); err != nil {
		return fmt.Errorf("update cursor: %w", err)
	}

	if err := s.playlistRepo.SkipPendingForSeries(ctx, playlistID, seriesID); err != nil {
		return fmt.Errorf("skip pending queue items: %w", err)
	}

	return nil
}

func selectFillCandidate(candidates []fillCandidate, slotType string, position, seriesCount int) (fillCandidate, bool) {
	if len(candidates) == 0 {
		return fillCandidate{}, false
	}

	pool := append([]fillCandidate(nil), candidates...)
	sort.Slice(pool, func(i, j int) bool {
		return pool[i].seriesID < pool[j].seriesID
	})

	switch slotType {
	case "least_recently_seen":
		sort.Slice(pool, func(i, j int) bool {
			if pool[i].lastSeenAt == nil || pool[j].lastSeenAt == nil {
				return pool[i].lastSeenAt == nil && pool[j].lastSeenAt != nil
			}
			if !pool[i].lastSeenAt.Equal(*pool[j].lastSeenAt) {
				return pool[i].lastSeenAt.Before(*pool[j].lastSeenAt)
			}
			return pool[i].seriesID < pool[j].seriesID
		})
		return pool[0], true
	case "top_rated":
		topPoolSize := seriesCount/2 + 1
		pool = ratedFillCandidates(pool)
		if len(pool) == 0 {
			return fillCandidate{}, false
		}
		sort.Slice(pool, func(i, j int) bool {
			if pool[i].rating != pool[j].rating {
				return pool[i].rating > pool[j].rating
			}
			return pool[i].seriesID < pool[j].seriesID
		})
		// Pick from the top half of attached series, plus one, so highly rated
		// episodes remain favored without repeatedly selecting the same top entry.
		if topPoolSize > len(pool) {
			topPoolSize = len(pool)
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(topPoolSize)))
		if err != nil {
			return fillCandidate{}, false
		}
		return pool[n.Int64()], true
	case "lowest_rated":
		pool = ratedFillCandidates(pool)
		if len(pool) == 0 {
			return fillCandidate{}, false
		}
		sort.Slice(pool, func(i, j int) bool {
			if pool[i].rating != pool[j].rating {
				return pool[i].rating < pool[j].rating
			}
			return pool[i].seriesID < pool[j].seriesID
		})
		return pool[0], true
	default:
		idx := position % len(pool)
		return pool[idx], true
	}
}

func ratedFillCandidates(candidates []fillCandidate) []fillCandidate {
	rated := make([]fillCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.rating > 0 {
			rated = append(rated, candidate)
		}
	}
	return rated
}

func (s *Service) PublishPlaylist(ctx context.Context, playlistID string) error {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	// Fill queue to target before publishing
	if _, err := s.fillPlaylist(ctx, playlistID); err != nil {
		return fmt.Errorf("fill before publish: %w", err)
	}
	return s.publishPlaylistProjection(ctx, playlistID)
}

// publishPlaylistProjection makes Plex exactly match the active local queue.
func (s *Service) publishPlaylistProjection(ctx context.Context, playlistID string) error {
	p, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return err
	}

	server, err := s.serverRepo.GetByID(ctx, p.MediaServerID)
	if err != nil {
		return fmt.Errorf("get server: %w", err)
	}

	items, err := s.playlistRepo.ListPendingQueueItems(ctx, playlistID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		binding, err := s.queueBindingRepo.GetByPlaylist(ctx, playlistID)
		if err != nil || binding == nil || binding.ServerPlaylistID == nil || *binding.ServerPlaylistID == "" {
			return nil
		}
		if err := plex.NewClient(server.URL, server.Token, 30*time.Second).ClearPlaylistItems(ctx, *binding.ServerPlaylistID); err != nil {
			return fmt.Errorf("clear Plex playlist: %w", err)
		}
		return nil
	}

	client := plex.NewClient(server.URL, server.Token, 30*time.Second)

	episodeIDs := make([]string, len(items))
	seenPlexIDs := make(map[string]string, len(items))
	duplicatePlexIDs := make([]string, 0)
	for i, item := range items {
		ep, err := s.episodeRepo.GetByID(ctx, item.EpisodeID)
		if err != nil {
			return fmt.Errorf("get episode %s: %w", item.EpisodeID, err)
		}
		episodeIDs[i] = ep.ServerEpisodeID
		series, _ := s.seriesRepo.GetByID(ctx, item.SeriesID)
		seriesTitle := ""
		if series != nil {
			seriesTitle = series.Title
		}
		if prior, duplicate := seenPlexIDs[ep.ServerEpisodeID]; duplicate {
			duplicatePlexIDs = append(duplicatePlexIDs, ep.ServerEpisodeID)
			slog.Error("duplicate Plex episode ID in intended projection", "playlist_id", playlistID, "plex_episode_id", ep.ServerEpisodeID, "queue_item_id", item.ID, "duplicate_queue_item_id", prior)
		} else {
			seenPlexIDs[ep.ServerEpisodeID] = item.ID
		}
		slog.Info("playlist projection queue item", "playlist_id", playlistID, "position", item.Position, "queue_item_id", item.ID, "local_episode_id", item.EpisodeID, "plex_episode_id", ep.ServerEpisodeID, "series_title", seriesTitle, "episode_title", ep.Title)
	}
	if len(duplicatePlexIDs) > 0 {
		return fmt.Errorf("duplicate Plex episode IDs in intended projection: %v", duplicatePlexIDs)
	}
	slog.Info("playlist projection submit", "playlist_id", playlistID, "requested_count", len(episodeIDs), "requested_plex_episode_ids", episodeIDs)

	// Look up existing binding for this queue playlist
	var playlistIDPtr *string
	binding, err := s.queueBindingRepo.GetByPlaylist(ctx, playlistID)
	if err == nil && binding != nil {
		playlistIDPtr = binding.ServerPlaylistID
	}

	plexPlaylist, err := client.UpsertPlaylist(ctx, playlistIDPtr, p.PlexPlaylistName, episodeIDs)
	if err != nil {
		if mismatch, ok := err.(*plex.PublicationMismatchError); ok {
			slog.Error("playlist projection verification failed", "playlist_id", playlistID, "expected_count", len(mismatch.Expected), "actual_count", len(mismatch.Actual), "expected_plex_episode_ids", mismatch.Expected, "actual_plex_episode_ids", mismatch.Actual, "missing_plex_episode_ids", mismatch.Missing, "unexpected_plex_episode_ids", mismatch.Unexpected, "order_mismatches", mismatch.OrderMismatches)
		}
		return fmt.Errorf("upsert plex playlist: %w", err)
	}
	readback, err := client.ListPlaylistItems(ctx, plexPlaylist.ID)
	if err != nil {
		return fmt.Errorf("read back verified Plex playlist: %w", err)
	}
	actualIDs := make([]string, len(readback))
	for i, item := range readback {
		actualIDs[i] = item.EpisodeID
	}
	slog.Info("playlist projection readback", "playlist_id", playlistID, "plex_playlist_id", plexPlaylist.ID, "actual_count", len(actualIDs), "actual_plex_episode_ids", actualIDs)

	now := time.Now()
	bindingRecord := &repository.QueuePlaylistBinding{
		ID:               uuid.New().String(),
		PlaylistID:       playlistID,
		MediaServerID:    server.ID,
		ServerPlaylistID: &plexPlaylist.ID,
		PlaylistName:     p.PlexPlaylistName,
		SynchronizedAt:   &now,
	}
	if err := s.queueBindingRepo.Upsert(ctx, bindingRecord); err != nil {
		return fmt.Errorf("save playlist binding: %w", err)
	}

	// Mark items as pushed after successful publish
	if err := s.playlistRepo.MarkPushed(ctx, playlistID); err != nil {
		slog.Warn("mark pushed failed", "playlist_id", playlistID, "error", err)
	}

	return nil
}

// ClearPlaylistQueue removes both the local queue and its derived Plex playlist.
func (s *Service) ClearPlaylistQueue(ctx context.Context, playlistID string) error {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	return s.clearPlaylistQueue(ctx, playlistID)
}

func (s *Service) clearPlaylistQueue(ctx context.Context, playlistID string) error {
	p, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return err
	}
	binding, err := s.queueBindingRepo.GetByPlaylist(ctx, playlistID)
	if err == nil && binding != nil && binding.ServerPlaylistID != nil && *binding.ServerPlaylistID != "" {
		server, err := s.serverRepo.GetByID(ctx, p.MediaServerID)
		if err != nil {
			return fmt.Errorf("get server: %w", err)
		}
		if err := plex.NewClient(server.URL, server.Token, 30*time.Second).ClearPlaylistItems(ctx, *binding.ServerPlaylistID); err != nil {
			return fmt.Errorf("clear Plex playlist: %w", err)
		}
	}
	if err := s.playlistRepo.ClearQueue(ctx, playlistID); err != nil {
		return fmt.Errorf("clear queue: %w", err)
	}
	return nil
}

func (s *Service) RefillPlaylist(ctx context.Context, playlistID string) (int, error) {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	if err := s.clearPlaylistQueue(ctx, playlistID); err != nil {
		return 0, err
	}
	queued, err := s.fillPlaylist(ctx, playlistID)
	if err != nil {
		return 0, fmt.Errorf("fill cleared queue: %w", err)
	}
	if queued == 0 {
		return 0, nil
	}
	if err := s.publishPlaylistProjection(ctx, playlistID); err != nil {
		return 0, fmt.Errorf("publish refilled queue: %w", err)
	}
	return queued, nil
}

func (s *Service) GetPlexPlaylist(ctx context.Context, playlistID string) (*PlexPlaylistResponse, error) {
	p, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return nil, err
	}
	binding, err := s.queueBindingRepo.GetByPlaylist(ctx, playlistID)
	if err != nil || binding.ServerPlaylistID == nil || *binding.ServerPlaylistID == "" {
		return nil, fmt.Errorf("playlist has not been published to Plex")
	}
	server, err := s.serverRepo.GetByID(ctx, p.MediaServerID)
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}

	items, err := plex.NewClient(server.URL, server.Token, 30*time.Second).ListPlaylistItems(ctx, *binding.ServerPlaylistID)
	if err != nil {
		return nil, err
	}
	response := &PlexPlaylistResponse{Items: make([]PlexPlaylistItemResponse, 0, len(items))}
	active, _ := s.playlistRepo.ListPendingQueueItems(ctx, playlistID)
	byPlexID := make(map[string]string, len(active))
	for _, queued := range active {
		if ep, err := s.episodeRepo.GetByID(ctx, queued.EpisodeID); err == nil {
			byPlexID[ep.ServerEpisodeID] = queued.ID
		}
	}
	for _, item := range items {
		response.Items = append(response.Items, PlexPlaylistItemResponse{
			QueueItemID:     byPlexID[item.EpisodeID],
			ServerEpisodeID: item.EpisodeID,
			SeriesTitle:     item.SeriesTitle,
			EpisodeTitle:    item.EpisodeTitle,
			SeasonNumber:    item.SeasonNumber,
			EpisodeNumber:   item.EpisodeNumber,
		})
	}
	response.BaseRevision = queueRevision(active)
	return response, nil
}

func (s *Service) ReplacePlexPlaylist(ctx context.Context, playlistID, baseRevision string, orderedItemIDs, removedItemIDs []string) error {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	return s.replacePlexPlaylist(ctx, playlistID, baseRevision, orderedItemIDs, removedItemIDs)
}

func (s *Service) replacePlexPlaylist(ctx context.Context, playlistID, baseRevision string, orderedItemIDs, removedItemIDs []string) error {
	p, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return err
	}
	binding, err := s.queueBindingRepo.GetByPlaylist(ctx, playlistID)
	if err != nil || binding == nil || binding.ServerPlaylistID == nil || *binding.ServerPlaylistID == "" {
		return fmt.Errorf("playlist has not been published to Plex")
	}

	active, err := s.playlistRepo.ListPendingQueueItems(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("list active queue: %w", err)
	}
	currentRevision := queueRevision(active)
	if baseRevision != currentRevision {
		return &StalePlaylistEditError{Expected: baseRevision, Actual: currentRevision}
	}
	byID := make(map[string]*repository.PlaylistQueueItem, len(active))
	for i := range active {
		byID[active[i].ID] = &active[i]
	}
	retained := make(map[string]bool, len(orderedItemIDs))
	for _, itemID := range orderedItemIDs {
		if retained[itemID] {
			return fmt.Errorf("queue item %s appears more than once", itemID)
		}
		if _, ok := byID[itemID]; !ok {
			return fmt.Errorf("queue item %s is not in the editing snapshot", itemID)
		}
		retained[itemID] = true
	}
	removed := make(map[string]bool, len(removedItemIDs))
	for _, itemID := range removedItemIDs {
		if removed[itemID] {
			return fmt.Errorf("queue item %s appears more than once in removals", itemID)
		}
		if _, ok := byID[itemID]; !ok {
			return fmt.Errorf("queue item %s is not in the editing snapshot", itemID)
		}
		if retained[itemID] {
			return fmt.Errorf("queue item %s is both retained and removed", itemID)
		}
		removed[itemID] = true
	}
	if len(retained)+len(removed) != len(active) {
		return fmt.Errorf("editing snapshot must explicitly retain or remove every active queue item")
	}
	for itemID := range removed {
		if err := s.completePlaylistQueueItem(ctx, p, byID[itemID]); err != nil {
			return fmt.Errorf("complete explicitly removed queue item %s: %w", itemID, err)
		}
	}
	if err := s.playlistRepo.ReorderActiveQueueItems(ctx, playlistID, orderedItemIDs); err != nil {
		return fmt.Errorf("reorder queue: %w", err)
	}
	if _, err := s.fillPlaylist(ctx, playlistID); err != nil {
		return fmt.Errorf("refill reconciled queue: %w", err)
	}
	if err := s.publishPlaylistProjection(ctx, playlistID); err != nil {
		return fmt.Errorf("publish reconciled queue: %w", err)
	}
	return nil
}

// completePlaylistQueueItem is the only path for consuming an active item.
// Consumption is local to this playlist and never changes Plex watch state.
func (s *Service) completePlaylistQueueItem(ctx context.Context, playlist *repository.Playlist, item *repository.PlaylistQueueItem) error {
	playlistSeriesID := item.PlaylistSeriesID
	if playlistSeriesID == nil {
		id, err := s.findPlaylistSeriesID(ctx, playlist.ID, item.SeriesID)
		if err != nil {
			return err
		}
		playlistSeriesID = &id
	}
	var nextID *string
	var nextPos *int
	if item.PlaylistSeriesID != nil {
		// A queued look-ahead episode is valid to consume without moving the
		// cursor. Only the current cursor episode advances progression.
		progress, err := s.playlistRepo.GetProgress(ctx, *item.PlaylistSeriesID)
		if err != nil {
			return fmt.Errorf("get playlist progress: %w", err)
		}
		if progress != nil && progress.NextEpisodeID != nil && *progress.NextEpisodeID == item.EpisodeID {
			member, err := s.playlistRepo.GetPlaylistSeriesByID(ctx, *item.PlaylistSeriesID)
			if err != nil {
				return fmt.Errorf("get playlist series: %w", err)
			}
			episodes, err := s.episodeRepo.ListBySeries(ctx, member.SeriesID)
			if err != nil {
				return err
			}
			rules, err := s.profileRules(ctx, member.ShowProfileID)
			if err != nil {
				return err
			}
			for i := range episodes {
				if episodes[i].ID == item.EpisodeID {
					for j := i + 1; j < len(episodes); j++ {
						if !episodes[j].Unavailable && rules.Allows(episodes[j]) {
							nextID = &episodes[j].ID
							nextPos = &episodes[j].AbsoluteOrder
							break
						}
					}
					break
				}
			}
		}
	}
	completed, err := s.playlistRepo.CompleteQueueItem(ctx, item.ID, *playlistSeriesID, item.EpisodeID, nextID, nextPos)
	if err != nil {
		return err
	}
	if !completed {
		return nil
	}
	return nil
}

func (s *Service) SyncPlaylist(ctx context.Context, playlistID string) (int, int, error) {
	lock := s.playlistLock(playlistID)
	lock.Lock()
	defer lock.Unlock()
	return s.syncPlaylist(ctx, playlistID)
}

func (s *Service) syncPlaylist(ctx context.Context, playlistID string) (int, int, error) {
	p, err := s.playlistRepo.GetByID(ctx, playlistID)
	if err != nil {
		return 0, 0, err
	}

	items, err := s.playlistRepo.ListQueueItems(ctx, playlistID)
	if err != nil {
		return 0, 0, err
	}

	server, err := s.serverRepo.GetByID(ctx, p.MediaServerID)
	if err != nil {
		return 0, 0, fmt.Errorf("get server: %w", err)
	}

	client := plex.NewClient(server.URL, server.Token, 30*time.Second)

	episodeIDs := make([]string, 0, len(items))
	itemByServerID := make(map[string]*repository.PlaylistQueueItem)

	for _, item := range items {
		if item.Status == "watched" || item.Status == "skipped" {
			continue
		}
		ep, err := s.episodeRepo.GetByID(ctx, item.EpisodeID)
		if err != nil {
			continue
		}
		episodeIDs = append(episodeIDs, ep.ServerEpisodeID)
		itemByServerID[ep.ServerEpisodeID] = &item
	}

	progressList, err := client.GetEpisodeProgress(ctx, episodeIDs)
	if err != nil {
		return 0, 0, fmt.Errorf("get progress: %w", err)
	}

	watched := 0
	for _, progress := range progressList {
		item, ok := itemByServerID[progress.EpisodeID]
		if !ok {
			continue
		}
		if progress.Watching && item.Status != "watching" {
			if err := s.playlistRepo.UpdateItemStatus(ctx, item.ID, "watching"); err != nil {
				return watched, 0, fmt.Errorf("update watching item %s: %w", item.ID, err)
			}
			continue
		}
		newlyViewed := newlyViewedAfter(progress, item.CreatedAt)
		if !newlyViewed {
			continue
		}

		if err := s.completePlaylistQueueItem(ctx, p, item); err != nil {
			return watched, 0, fmt.Errorf("complete playlist queue item %s: %w", item.ID, err)
		}
		watched++
	}

	queued, err := s.fillPlaylist(ctx, playlistID)
	if err != nil {
		return watched, 0, fmt.Errorf("refill queue: %w", err)
	}
	// Plex is only a projection of this queue. Apply every sync, even when no item
	// changed, so a prior Plex failure is retried and removed episodes cannot linger.
	if err := s.publishPlaylistProjection(ctx, playlistID); err != nil {
		return watched, queued, fmt.Errorf("publish queue projection: %w", err)
	}

	return watched, queued, nil
}

func newlyViewedAfter(progress media.EpisodeProgress, queuedAt time.Time) bool {
	return progress.ViewCount > 0 &&
		progress.LastViewedAt > 0 &&
		time.Unix(progress.LastViewedAt, 0).After(queuedAt)
}

func (s *Service) SyncEnabledPlaylists(ctx context.Context) error {
	playlists, err := s.playlistRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list playlists: %w", err)
	}
	for _, playlist := range playlists {
		if !playlist.Enabled {
			continue
		}
		if _, _, err := s.SyncPlaylist(ctx, playlist.ID); err != nil {
			slog.Warn("sync playlist failed", "playlist_id", playlist.ID, "error", err)
		}
	}
	return nil
}

func (s *Service) findPlaylistSeriesID(ctx context.Context, playlistID, seriesID string) (string, error) {
	members, err := s.playlistRepo.ListSeries(ctx, playlistID)
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.SeriesID == seriesID {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("series not in playlist")
}
