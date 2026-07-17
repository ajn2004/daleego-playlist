package media

import "context"

type Library struct {
	ID    string
	Title string
	Type  string
}

type SeriesMetadata struct {
	ID          string
	Title       string
	Summary     string
	Year        int
	LibraryID   string
	LibraryName string
}

type EpisodeMetadata struct {
	ID            string
	SeriesID      string
	Title         string
	SeasonNumber  int
	EpisodeNumber int
	AbsoluteOrder int
	Duration      int
	Rating        float64
	AirDate       string
}

type EpisodeProgress struct {
	EpisodeID  string
	Viewed     bool
	Watching   bool
	ViewCount  int
	ViewOffset int
}

type Playlist struct {
	ID      string
	Name    string
	ItemIDs []string
}

type PlaylistItem struct {
	EpisodeID     string
	SeriesTitle   string
	EpisodeTitle  string
	SeasonNumber  int
	EpisodeNumber int
}

type MediaServer interface {
	TestConnection(ctx context.Context) error
	ListLibraries(ctx context.Context) ([]Library, error)
	ListSeries(ctx context.Context, libraryID string) ([]SeriesMetadata, error)
	ListEpisodes(ctx context.Context, seriesID string) ([]EpisodeMetadata, error)
	GetEpisodeProgress(ctx context.Context, episodeIDs []string) ([]EpisodeProgress, error)
	ListPlaylistItems(ctx context.Context, playlistID string) ([]PlaylistItem, error)
	ClearPlaylistItems(ctx context.Context, playlistID string) error
	UpsertPlaylist(ctx context.Context, playlistID *string, name string, episodeIDs []string) (Playlist, error)
}
