package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL      string
	HTTPAddr         string
	PlexURL          string
	PlexToken        string
	PlexPlaylistName string
	LogLevel         string
	SyncInterval     time.Duration
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	httpAddr := envDefault("HTTP_ADDR", ":8091")
	plexURL := os.Getenv("PLEX_URL")
	plexToken := os.Getenv("PLEX_TOKEN")
	plexPlaylistName := envDefault("PLEX_PLAYLIST_NAME", "TV Rotation")
	logLevel := envDefault("LOG_LEVEL", "info")
	syncInterval := envDefaultDuration("SYNC_INTERVAL", 60*time.Second)

	return &Config{
		DatabaseURL:      dbURL,
		HTTPAddr:         httpAddr,
		PlexURL:          plexURL,
		PlexToken:        plexToken,
		PlexPlaylistName: plexPlaylistName,
		LogLevel:         logLevel,
		SyncInterval:     syncInterval,
	}, nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDefaultDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}

func envDefaultInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}
