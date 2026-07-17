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
