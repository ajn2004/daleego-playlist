package service

import (
	"testing"

	"github.com/andrew/rotator/internal/repository"
)

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
