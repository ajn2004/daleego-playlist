package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Show struct {
	ServerID     string
	ServerShowID string
	GUID         string
	LibraryID    string
	Title        string
}

type Response struct {
	Shows      []Show
	NextCursor string
}

type Source interface {
	FetchNewShows(context.Context, string) (Response, error)
}
type SourceFunc func(context.Context, string) (Response, error)

func (f SourceFunc) FetchNewShows(ctx context.Context, cursor string) (Response, error) {
	return f(ctx, cursor)
}

type Catalog interface {
	UpsertFromServer(context.Context, Show) (added bool, err error)
}
type CatalogFunc func(context.Context, Show) (bool, error)

func (f CatalogFunc) UpsertFromServer(ctx context.Context, show Show) (bool, error) {
	return f(ctx, show)
}

type State interface {
	DiscoveryCursor(context.Context) (string, error)
	RecordPollAttempt(context.Context, time.Time) error
	RecordPollSuccess(context.Context, string, time.Time) error
	RecordPollFailure(context.Context, time.Time, string) error
}

type Status struct {
	Cursor      string     `json:"cursor"`
	LastAttempt *time.Time `json:"last_attempt"`
	LastSuccess *time.Time `json:"last_success"`
	LastError   *string    `json:"last_error"`
}

type Service struct {
	source  Source
	catalog Catalog
	state   State
	now     func() time.Time
	logger  *slog.Logger
	mu      sync.Mutex
}

func New(source Source, catalog Catalog, state State, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{source: source, catalog: catalog, state: state, now: time.Now, logger: logger}
}

func (s *Service) DiscoverNewShows(ctx context.Context) error {
	if !s.mu.TryLock() {
		return fmt.Errorf("show discovery already running")
	}
	defer s.mu.Unlock()

	started := s.now()
	cursor, err := s.state.DiscoveryCursor(ctx)
	if err != nil {
		return fmt.Errorf("read discovery cursor: %w", err)
	}
	if err := s.state.RecordPollAttempt(ctx, started); err != nil {
		return fmt.Errorf("record discovery attempt: %w", err)
	}
	s.logger.Info("discovery poll started", "cursor", cursor)

	response, err := s.source.FetchNewShows(ctx, cursor)
	if err != nil {
		return s.failed(ctx, started, fmt.Errorf("fetch new shows: %w", err))
	}
	added := 0
	for _, show := range response.Shows {
		isNew, err := s.catalog.UpsertFromServer(ctx, show)
		if err != nil {
			return s.failed(ctx, started, fmt.Errorf("process show %q: %w", show.ServerShowID, err))
		}
		if isNew {
			added++
		}
	}
	completed := s.now()
	if err := s.state.RecordPollSuccess(ctx, response.NextCursor, completed); err != nil {
		return fmt.Errorf("save discovery cursor: %w", err)
	}
	s.logger.Info("discovery poll completed", "received", len(response.Shows), "added", added, "updated", len(response.Shows)-added, "duration_ms", completed.Sub(started).Milliseconds())
	return nil
}

func (s *Service) failed(ctx context.Context, at time.Time, err error) error {
	if stateErr := s.state.RecordPollFailure(ctx, s.now(), err.Error()); stateErr != nil {
		s.logger.Error("discovery poll failed", "error", err, "record_error", stateErr)
	} else {
		s.logger.Error("discovery poll failed", "error", err)
	}
	return err
}

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	if err := s.DiscoverNewShows(ctx); err != nil {
		s.logger.Warn("initial show discovery failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.DiscoverNewShows(ctx); err != nil {
				s.logger.Warn("show discovery failed", "error", err)
			}
		}
	}
}
