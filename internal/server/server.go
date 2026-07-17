package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/andrew/rotator/db/migrations"
	"github.com/andrew/rotator/internal/config"
	"github.com/andrew/rotator/internal/media/plex"
	"github.com/andrew/rotator/internal/repository"
	"github.com/andrew/rotator/internal/rotation"
	"github.com/andrew/rotator/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	plexClient *plex.Client
	router     http.Handler
	repos      *repository.Queries
	svc        *service.Service
}

func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	repos := repository.New(pool)

	plexClient := plex.NewClient(cfg.PlexURL, cfg.PlexToken, 30*time.Second)

	policyRepo := repository.NewPolicyRepo(pool)
	rotationRepo := repository.NewRotationRepo(pool)
	seriesRepo := repository.NewSeriesRepo(pool)
	episodeRepo := repository.NewEpisodeRepo(pool)
	progressRepo := repository.NewProgressRepo(pool)
	bindingRepo := repository.NewBindingRepo(pool)
	playlistRepo := repository.NewPlaylistRepo(pool)
	serverRepo := repository.NewServerRepo(pool)
	queueBindingRepo := repository.NewQueueBindingRepo(pool)

	svc := service.New(
		cfg.PlexPlaylistName,
		plexClient,
		serverRepo,
		seriesRepo,
		episodeRepo,
		progressRepo,
		policyRepo,
		rotationRepo,
		bindingRepo,
		playlistRepo,
		queueBindingRepo,
		rotation.NewEngine(),
	)

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		plexClient: plexClient,
		repos:      repos,
		svc:        svc,
	}

	s.router = s.buildRouter()

	return s, nil
}

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReady)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", s.handleStatus)

		r.Get("/media-servers", s.handleListServers)
		r.Post("/media-servers", s.handleCreateServer)
		r.Post("/media-servers/{id}/test", s.handleTestServer)

		r.Get("/series", s.handleListSeries)
		r.Get("/series/{id}", s.handleGetSeries)
		r.Patch("/series/{id}", s.handleUpdateSeries)
		r.Post("/series/{id}/sync", s.handleSyncSeries)
		r.Post("/series/{id}/reconcile", s.handleReconcileSeries)

		r.Get("/rotation-profiles", s.handleListProfiles)
		r.Post("/rotation-profiles", s.handleCreateProfile)
		r.Put("/rotation-profiles/{id}", s.handleUpdateProfile)
		r.Post("/rotation-profiles/{id}/preview", s.handlePreviewProfile)

		r.Get("/rotations/current", s.handleCurrentRotation)
		r.Post("/rotations/generate", s.handleGenerateRotation)
		r.Post("/rotations/{id}/publish", s.handlePublishRotation)
		r.Post("/rotations/{id}/reroll", s.handleRerollRotation)
		r.Post("/rotations/{id}/sync", s.handleSyncRotation)

		r.Get("/playlists", s.handleListPlaylists)
		r.Post("/playlists", s.handleCreatePlaylist)
		r.Get("/playlists/{id}", s.handleGetPlaylist)
		r.Patch("/playlists/{id}", s.handleUpdatePlaylist)
		r.Delete("/playlists/{id}", s.handleDeletePlaylist)
		r.Put("/playlists/{id}/series", s.handleSetPlaylistSeries)
		r.Put("/playlists/{id}/slots", s.handleSetPlaylistSlots)
		r.Post("/playlists/{id}/fill", s.handleFillPlaylist)
		r.Post("/playlists/{id}/clear", s.handleClearPlaylist)
		r.Post("/playlists/{id}/refill", s.handleRefillPlaylist)
		r.Post("/playlists/{id}/publish", s.handlePublishPlaylist)
		r.Post("/playlists/{id}/sync", s.handleSyncPlaylist)
		r.Get("/playlists/{id}/plex-items", s.handleGetPlexPlaylist)
		r.Put("/playlists/{id}/plex-items", s.handleReplacePlexPlaylist)
		r.Get("/playlists/{playlistID}/series/{seriesID}/episodes", s.handleListPlaylistSeriesEpisodes)
		r.Post("/playlists/{playlistID}/series/{seriesID}/cursor", s.handleSetPlaylistNextEpisode)
	})

	return r
}

func (s *Server) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.cfg.HTTPAddr,
		Handler: s.router,
	}

	// Run sync loop
	go func() {
		if err := s.svc.SyncEnabledPlaylists(ctx); err != nil {
			slog.Warn("initial playlist sync failed", "error", err)
		}
		ticker := time.NewTicker(s.cfg.SyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.svc.SyncRotation(ctx); err != nil {
					slog.Warn("sync rotation failed", "error", err)
				}
				if err := s.svc.SyncEnabledPlaylists(ctx); err != nil {
					slog.Warn("sync playlists failed", "error", err)
				}
			}
		}
	}()

	slog.Info("server starting", "addr", s.cfg.HTTPAddr)

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Migrate(ctx context.Context) error {
	db, err := sql.Open("pgx", s.cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect migration database: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	return nil
}

func (s *Server) SyncAll(ctx context.Context) error {
	return s.svc.SyncAll(ctx)
}

func (s *Server) GenerateRotation(ctx context.Context) error {
	_, err := s.svc.GenerateRotation(ctx)
	return err
}

func (s *Server) PublishRotation(ctx context.Context) error {
	return s.svc.PublishCurrentRotation(ctx)
}
