package migrations

import "testing"

func TestEmbeddedMigrationsIncludeQueueBindings(t *testing.T) {
	contents, err := FS.ReadFile("003_queue_bindings.sql")
	if err != nil {
		t.Fatalf("read queue bindings migration: %v", err)
	}

	if len(contents) == 0 {
		t.Fatal("queue bindings migration is empty")
	}
}

func TestEmbeddedMigrationsIncludeRandomEpisodeCooldown(t *testing.T) {
	contents, err := FS.ReadFile("006_random_episode_cooldown.sql")
	if err != nil {
		t.Fatalf("read random episode cooldown migration: %v", err)
	}
	if len(contents) == 0 {
		t.Fatal("random episode cooldown migration is empty")
	}
}

func TestEmbeddedMigrationsIncludeSeriesServerGUID(t *testing.T) {
	contents, err := FS.ReadFile("010_series_server_guid.sql")
	if err != nil {
		t.Fatalf("read series GUID migration: %v", err)
	}
	if len(contents) == 0 {
		t.Fatal("series GUID migration is empty")
	}
}
