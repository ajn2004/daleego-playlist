package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/andrew/rotator/internal/config"
	"github.com/andrew/rotator/internal/media"
	"github.com/andrew/rotator/internal/media/plex"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	if cfg.PlexURL == "" || cfg.PlexToken == "" {
		fmt.Fprintln(os.Stderr, "PLEX_URL and PLEX_TOKEN are required")
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	client := plex.NewClient(cfg.PlexURL, cfg.PlexToken, 30*time.Second)

	ctx := context.Background()

	fmt.Println("=== Testing connection ===")
	if err := client.TestConnection(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "connection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK")

	fmt.Println("\n=== Listing libraries ===")
	libraries, err := client.ListLibraries(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list libraries failed: %v\n", err)
		os.Exit(1)
	}
	for _, lib := range libraries {
		fmt.Printf("  %s: %s (type=%s)\n", lib.ID, lib.Title, lib.Type)
	}

	if len(libraries) == 0 {
		fmt.Fprintln(os.Stderr, "no libraries found")
		os.Exit(1)
	}

	// Use first TV library
	var tvLib *media.Library
	for _, lib := range libraries {
		if lib.Type == "show" {
			tvLib = &lib
			break
		}
	}
	if tvLib == nil {
		fmt.Fprintln(os.Stderr, "no TV library found")
		os.Exit(1)
	}

	libraryID := tvLib.ID
	if len(os.Args) > 1 {
		libraryID = os.Args[1]
	}

	fmt.Printf("\n=== Listing series in library %s ===\n", libraryID)
	seriesList, err := client.ListSeries(ctx, libraryID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list series failed: %v\n", err)
		os.Exit(1)
	}
	for _, s := range seriesList {
		fmt.Printf("  %s: %s\n", s.ID, s.Title)
	}

	if len(seriesList) == 0 {
		fmt.Fprintln(os.Stderr, "no series found")
		os.Exit(1)
	}

	// List episodes for the first series
	firstSeries := seriesList[0]
	if len(os.Args) > 2 {
		for _, s := range seriesList {
			if s.Title == os.Args[2] || s.ID == os.Args[2] {
				firstSeries = s
				break
			}
		}
	}

	fmt.Printf("\n=== Listing episodes for %s ===\n", firstSeries.Title)
	episodes, err := client.ListEpisodes(ctx, firstSeries.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list episodes failed: %v\n", err)
		os.Exit(1)
	}
	for i, ep := range episodes {
		if i >= 20 {
			fmt.Printf("  ... %d more\n", len(episodes)-20)
			break
		}
		fmt.Printf("  S%02dE%02d: %s (rating=%.1f, duration=%ds, key=%s)\n",
			ep.SeasonNumber, ep.EpisodeNumber, ep.Title, ep.Rating, ep.Duration, ep.ID)
	}

	// Create a test playlist with first 3 episodes
	if len(episodes) >= 3 {
		fmt.Println("\n=== Creating test playlist ===")
		episodeIDs := make([]string, 3)
		for i := 0; i < 3; i++ {
			episodeIDs[i] = episodes[i].ID
		}
		playlist, err := client.UpsertPlaylist(ctx, nil, "TV Rotation Test", episodeIDs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create playlist failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Playlist %s created with %d items\n", playlist.ID, len(playlist.ItemIDs))
	}

	os.Exit(0)
}
