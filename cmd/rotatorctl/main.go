package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/andrew/rotator/internal/config"
	"github.com/andrew/rotator/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx := context.Background()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	switch {
	case len(os.Args) > 1 && os.Args[1] == "migrate":
		if err := srv.Migrate(ctx); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("migrations complete")
	case len(os.Args) > 1 && os.Args[1] == "sync":
		if err := srv.SyncAll(ctx); err != nil {
			slog.Error("sync failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("sync complete")
	case len(os.Args) > 1 && os.Args[1] == "generate":
		if err := srv.GenerateRotation(ctx); err != nil {
			slog.Error("generate failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("rotation generated")
	case len(os.Args) > 1 && os.Args[1] == "publish":
		if err := srv.PublishRotation(ctx); err != nil {
			slog.Error("publish failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("rotation published")
	default:
		fmt.Println("usage: rotatorctl {migrate|sync|generate|publish}")
	}
}
