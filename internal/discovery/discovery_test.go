package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryState struct {
	cursor                        string
	attempts, successes, failures int
	lastErr                       string
}

func (s *memoryState) DiscoveryCursor(context.Context) (string, error)    { return s.cursor, nil }
func (s *memoryState) RecordPollAttempt(context.Context, time.Time) error { s.attempts++; return nil }
func (s *memoryState) RecordPollSuccess(_ context.Context, cursor string, _ time.Time) error {
	s.cursor = cursor
	s.successes++
	return nil
}
func (s *memoryState) RecordPollFailure(_ context.Context, _ time.Time, err string) error {
	s.failures++
	s.lastErr = err
	return nil
}

func TestDiscoverNewShowsAdvancesOnlyAfterAllShows(t *testing.T) {
	state := &memoryState{cursor: "old"}
	processed := 0
	svc := New(SourceFunc(func(_ context.Context, cursor string) (Response, error) {
		if cursor != "old" {
			t.Fatalf("cursor = %q", cursor)
		}
		return Response{Shows: []Show{{ServerShowID: "one"}, {ServerShowID: "two"}}, NextCursor: "new"}, nil
	}), CatalogFunc(func(_ context.Context, show Show) (bool, error) {
		processed++
		if show.ServerShowID == "two" {
			return false, errors.New("broken show")
		}
		return true, nil
	}), state, nil)
	if err := svc.DiscoverNewShows(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if state.cursor != "old" || state.successes != 0 || state.failures != 1 || processed != 2 {
		t.Fatalf("state = %+v processed=%d", state, processed)
	}
}

func TestDiscoverNewShowsIsIdempotentAtCatalogBoundary(t *testing.T) {
	state := &memoryState{}
	seen := map[string]bool{}
	svc := New(SourceFunc(func(context.Context, string) (Response, error) {
		return Response{Shows: []Show{{ServerShowID: "one"}}, NextCursor: "next"}, nil
	}), CatalogFunc(func(_ context.Context, show Show) (bool, error) {
		if seen[show.ServerShowID] {
			return false, nil
		}
		seen[show.ServerShowID] = true
		return true, nil
	}), state, nil)
	if err := svc.DiscoverNewShows(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.DiscoverNewShows(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || state.successes != 2 {
		t.Fatalf("seen=%d successes=%d", len(seen), state.successes)
	}
}

func TestDiscoverNewShowsDoesNotOverlap(t *testing.T) {
	state := &memoryState{}
	started := make(chan struct{})
	release := make(chan struct{})
	svc := New(SourceFunc(func(context.Context, string) (Response, error) { close(started); <-release; return Response{}, nil }), CatalogFunc(func(context.Context, Show) (bool, error) { return false, nil }), state, nil)
	finished := make(chan error, 1)
	go func() { finished <- svc.DiscoverNewShows(context.Background()) }()
	<-started
	if err := svc.DiscoverNewShows(context.Background()); err == nil {
		t.Fatal("expected overlap error")
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestRunDiscoversImmediatelyAndStops(t *testing.T) {
	state := &memoryState{}
	var mu sync.Mutex
	calls := 0
	svc := New(SourceFunc(func(context.Context, string) (Response, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return Response{}, nil
	}), CatalogFunc(func(context.Context, Show) (bool, error) { return false, nil }), state, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.Run(ctx, 20*time.Millisecond); close(done) }()
	time.Sleep(5 * time.Millisecond)
	mu.Lock()
	initial := calls
	mu.Unlock()
	if initial != 1 {
		t.Fatalf("initial calls=%d", initial)
	}
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
	mu.Lock()
	final := calls
	mu.Unlock()
	if final < 2 {
		t.Fatalf("calls=%d", final)
	}
}
