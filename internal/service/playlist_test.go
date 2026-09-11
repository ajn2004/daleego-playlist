package service

import (
	"context"
	"testing"
	"time"

	"github.com/andrew/rotator/internal/media"
	"github.com/andrew/rotator/internal/repository"
)

func TestPlaylistLockWaiterHonorsCancellation(t *testing.T) {
	lock := newPlaylistLock()
	if err := lock.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer lock.release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := lock.acquire(ctx); err == nil {
		t.Fatal("expected canceled waiter to return without acquiring the lock")
	}
}

func TestNewlyViewedAfterIgnoresHistoricalPlexView(t *testing.T) {
	queuedAt := time.Unix(1700000100, 0)
	progress := media.EpisodeProgress{
		ViewCount:    3,
		LastViewedAt: 1700000000,
	}

	if newlyViewedAfter(progress, queuedAt) {
		t.Fatal("historical Plex view must not complete a newly queued item")
	}
	progress.LastViewedAt = 1700000101
	if !newlyViewedAfter(progress, queuedAt) {
		t.Fatal("view after queue creation should complete the queue item")
	}
}

func TestSelectFillCandidateEmpty(t *testing.T) {
	result, ok := selectFillCandidate(nil, "any", 0, 0)
	if ok || result.seriesID != "" {
		t.Error("expected empty candidate")
	}
}

func TestSelectFillCandidateSingle(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 8.0},
	}
	result, ok := selectFillCandidate(candidates, "any", 0, len(candidates))
	if !ok {
		t.Fatal("expected candidate to be selected")
	}
	if result.episodeID != "e1" {
		t.Error("expected only candidate to be selected")
	}
}

func TestSelectFillCandidateTopRated(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 5.0},
		{seriesID: "s2", episodeID: "e2", rating: 9.0},
		{seriesID: "s4", episodeID: "e4", rating: 8.0},
		{seriesID: "s3", episodeID: "e3", rating: 7.0},
	}
	result, ok := selectFillCandidate(candidates, "top_rated", 0, len(candidates))
	if !ok {
		t.Fatal("expected candidate to be selected")
	}
	if result.episodeID != "e2" && result.episodeID != "e4" && result.episodeID != "e3" {
		t.Errorf("expected an episode from the top three ratings, got: %s", result.episodeID)
	}
}

func TestSelectFillCandidateLowestRated(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 5.0},
		{seriesID: "s2", episodeID: "e2", rating: 9.0},
		{seriesID: "s3", episodeID: "e3", rating: 7.0},
	}
	result, ok := selectFillCandidate(candidates, "lowest_rated", 0, len(candidates))
	if !ok {
		t.Fatal("expected candidate to be selected")
	}
	if result.episodeID != "e1" {
		t.Error("expected lowest rated (e1) to be selected, got:", result.episodeID)
	}
}

func TestSelectFillCandidateAnyModulo(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 5.0},
		{seriesID: "s2", episodeID: "e2", rating: 9.0},
	}
	// With 2 candidates and position 0, should get first (after sort)
	result, ok := selectFillCandidate(candidates, "any", 0, len(candidates))
	if !ok {
		t.Fatal("expected candidate to be selected")
	}
	if result.episodeID != "e1" {
		t.Error("expected position 0 to select e1, got:", result.episodeID)
	}

	// With 2 candidates and position 1, should get second
	result, ok = selectFillCandidate(candidates, "any", 1, len(candidates))
	if !ok {
		t.Fatal("expected candidate to be selected")
	}
	if result.episodeID != "e2" {
		t.Error("expected position 1 to select e2, got:", result.episodeID)
	}

	// With 2 candidates and position 3, wraps around: 3 % 2 = 1
	result, ok = selectFillCandidate(candidates, "any", 3, len(candidates))
	if !ok {
		t.Fatal("expected candidate to be selected")
	}
	if result.episodeID != "e2" {
		t.Error("expected position 3 to wrap and select e2, got:", result.episodeID)
	}
}

func TestSelectFillCandidateLeastRecentlySeenPrioritizesNeverSeen(t *testing.T) {
	older := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	candidates := []fillCandidate{
		{seriesID: "recent", episodeID: "e1", lastSeenAt: &newer},
		{seriesID: "never", episodeID: "e2"},
		{seriesID: "older", episodeID: "e3", lastSeenAt: &older},
	}

	result, ok := selectFillCandidate(candidates, "least_recently_seen", 0, len(candidates))
	if !ok || result.episodeID != "e2" {
		t.Fatalf("expected never-seen candidate, got %#v", result)
	}
}

func TestSelectFillCandidateLeastRecentlySeenUsesOldestTimestamp(t *testing.T) {
	older := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	candidates := []fillCandidate{
		{seriesID: "recent", episodeID: "e1", lastSeenAt: &newer},
		{seriesID: "older", episodeID: "e2", lastSeenAt: &older},
	}

	result, ok := selectFillCandidate(candidates, "least_recently_seen", 0, len(candidates))
	if !ok || result.episodeID != "e2" {
		t.Fatalf("expected oldest candidate, got %#v", result)
	}
}

func TestSelectFillCandidateTopRatedExcludesLowerHalf(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 10.0},
		{seriesID: "s2", episodeID: "e2", rating: 9.0},
		{seriesID: "s3", episodeID: "e3", rating: 8.0},
		{seriesID: "s4", episodeID: "e4", rating: 7.0},
		{seriesID: "s5", episodeID: "e5", rating: 6.0},
	}

	for range 100 {
		result, ok := selectFillCandidate(candidates, "top_rated", 0, len(candidates))
		if !ok {
			t.Fatal("expected candidate to be selected")
		}
		if result.rating < 8.0 {
			t.Fatalf("expected an episode from the top three ratings, got %#v", result)
		}
	}
}

func TestSelectFillCandidateRatedSlotsExcludeUnavailableRatings(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "unrated", episodeID: "e1", rating: 0},
		{seriesID: "low", episodeID: "e2", rating: 6.5},
		{seriesID: "high", episodeID: "e3", rating: 9.3},
	}

	top, ok := selectFillCandidate(candidates, "top_rated", 0, len(candidates))
	if !ok || (top.episodeID != "e2" && top.episodeID != "e3") {
		t.Fatalf("expected an episode from the top two ratings, got %#v", top)
	}

	lowest, ok := selectFillCandidate(candidates, "lowest_rated", 0, len(candidates))
	if !ok || lowest.episodeID != "e2" {
		t.Fatalf("expected lowest rated episode e2, got %#v", lowest)
	}
}

func TestSelectFillCandidateRatedSlotsRequireRating(t *testing.T) {
	candidates := []fillCandidate{{seriesID: "unrated", episodeID: "e1", rating: 0}}

	if _, ok := selectFillCandidate(candidates, "top_rated", 0, len(candidates)); ok {
		t.Fatal("unrated episode must not fill a top-rated slot")
	}
	if _, ok := selectFillCandidate(candidates, "lowest_rated", 0, len(candidates)); ok {
		t.Fatal("unrated episode must not fill a lowest-rated slot")
	}
	if selected, ok := selectFillCandidate(candidates, "any", 0, len(candidates)); !ok || selected.episodeID != "e1" {
		t.Fatal("unrated episode should remain eligible for an any slot")
	}
}

func TestFirstUnqueuedEpisodeAtCursorLooksAhead(t *testing.T) {
	position := 2
	episodes := []repository.Episode{
		{ID: "e1", AbsoluteOrder: 1, Rating: 8.0},
		{ID: "e2", AbsoluteOrder: 2, Rating: 8.5},
		{ID: "e3", AbsoluteOrder: 3, Rating: 9.0},
	}

	got, ok := firstUnqueuedEpisodeAtCursor(episodes, "e2", &position, map[string]bool{"e2": true})
	if !ok || got.ID != "e3" {
		t.Fatalf("expected next unqueued episode e3, got %#v", got)
	}
}

func TestFirstUnqueuedEpisodeAtCursorSkipsConsumedHistory(t *testing.T) {
	position := 4
	episodes := []repository.Episode{
		{ID: "e4", AbsoluteOrder: 4, Rating: 8.0},
		{ID: "e5", AbsoluteOrder: 5, Rating: 8.5},
		{ID: "e6", AbsoluteOrder: 6, Rating: 9.0},
	}

	got, ok := firstAllowedUnqueuedEpisodeAtCursorWithHistory(
		episodes,
		"e4",
		&position,
		map[string]bool{"e4": true},
		map[string]bool{"e5": true},
		ShowProfileRules{DefaultAllow: true},
	)
	if !ok || got.ID != "e6" {
		t.Fatalf("expected consumed future episode e5 to be skipped, got %#v", got)
	}
}

func TestNextAllowedEpisodeAfterSkipsHistoryAndUnavailable(t *testing.T) {
	episodes := []repository.Episode{
		{ID: "e1", AbsoluteOrder: 1},
		{ID: "e2", AbsoluteOrder: 2},
		{ID: "e3", AbsoluteOrder: 3, Unavailable: true},
		{ID: "e4", AbsoluteOrder: 4},
	}

	got, ok := nextAllowedEpisodeAfter(episodes, "e1", ShowProfileRules{DefaultAllow: true}, map[string]bool{"e2": true})
	if !ok || got.ID != "e4" {
		t.Fatalf("expected e4 after skipping consumed e2 and unavailable e3, got %#v", got)
	}
}

func TestNextAllowedEpisodeAfterRequiresCurrentEpisode(t *testing.T) {
	episodes := []repository.Episode{{ID: "e1"}, {ID: "e2"}}
	if _, ok := nextAllowedEpisodeAfter(episodes, "missing", ShowProfileRules{DefaultAllow: true}, nil); ok {
		t.Fatal("must not advance when current episode is absent")
	}
}

func TestRemovedQueueItemsInOrderUsesActiveQueueOrder(t *testing.T) {
	active := []repository.PlaylistQueueItem{
		{ID: "e1"}, {ID: "e2"}, {ID: "e3"}, {ID: "e4"},
	}
	removed := map[string]bool{"e2": true, "e1": true}

	got := removedQueueItemsInOrder(active, removed)
	if len(got) != 2 || got[0].ID != "e1" || got[1].ID != "e2" {
		t.Fatalf("removed items were not returned in queue order: %#v", got)
	}
}

func TestEligibleRandomEpisodesUsesOnlyRecentHistory(t *testing.T) {
	episodes := []repository.Episode{{ID: "old"}, {ID: "recent"}, {ID: "queued"}, {ID: "available"}}
	got := eligibleRandomEpisodes(episodes, ShowProfileRules{DefaultAllow: true}, map[string]bool{"recent": true}, map[string]bool{"queued": true})
	if len(got) != 2 || got[0].ID != "old" || got[1].ID != "available" {
		t.Fatalf("expected old and available episodes to be eligible, got %#v", got)
	}
}

func TestEffectiveRandomEpisodeCooldownLeavesAnEpisodeAvailable(t *testing.T) {
	episodes := []repository.Episode{{ID: "e1"}, {ID: "e2"}, {ID: "e3"}}
	if got := effectiveRandomEpisodeCooldown(episodes, ShowProfileRules{DefaultAllow: true}, map[string]bool{"e3": true}, 10); got != 1 {
		t.Fatalf("expected cooldown to be capped at one available episode, got %d", got)
	}
}

func TestRemainingDurationStartsAtCursorAndRespectsProfile(t *testing.T) {
	episodes := []repository.Episode{
		{ID: "e1", AbsoluteOrder: 1, Duration: 1200},
		{ID: "e2", AbsoluteOrder: 2, Duration: 1500},
		{ID: "e3", AbsoluteOrder: 3, Duration: 1800},
	}
	rules := ShowProfileRules{
		DefaultAllow: true,
		Episodes:     map[string]bool{"e3": false},
	}

	got := remainingDuration(episodes, 2, rules)
	if got != 1500 {
		t.Fatalf("remaining duration = %d, want 1500", got)
	}
}
