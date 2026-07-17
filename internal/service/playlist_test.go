package service

import (
	"math/rand"
	"testing"
)

func TestSelectFillCandidateEmpty(t *testing.T) {
	result := selectFillCandidate(nil, "any", 0)
	if result.seriesID != "" {
		t.Error("expected empty candidate")
	}
}

func TestSelectFillCandidateSingle(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 8.0},
	}
	result := selectFillCandidate(candidates, "any", 0)
	if result.episodeID != "e1" {
		t.Error("expected only candidate to be selected")
	}
}

func TestSelectFillCandidateTopRated(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 5.0},
		{seriesID: "s2", episodeID: "e2", rating: 9.0},
		{seriesID: "s3", episodeID: "e3", rating: 7.0},
	}
	result := selectFillCandidate(candidates, "top_rated", 0)
	if result.episodeID != "e2" {
		t.Error("expected highest rated (e2) to be selected, got:", result.episodeID)
	}
}

func TestSelectFillCandidateLowestRated(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 5.0},
		{seriesID: "s2", episodeID: "e2", rating: 9.0},
		{seriesID: "s3", episodeID: "e3", rating: 7.0},
	}
	result := selectFillCandidate(candidates, "lowest_rated", 0)
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
	result := selectFillCandidate(candidates, "any", 0)
	if result.episodeID != "e1" {
		t.Error("expected position 0 to select e1, got:", result.episodeID)
	}

	// With 2 candidates and position 1, should get second
	result = selectFillCandidate(candidates, "any", 1)
	if result.episodeID != "e2" {
		t.Error("expected position 1 to select e2, got:", result.episodeID)
	}

	// With 2 candidates and position 3, wraps around: 3 % 2 = 1
	result = selectFillCandidate(candidates, "any", 3)
	if result.episodeID != "e2" {
		t.Error("expected position 3 to wrap and select e2, got:", result.episodeID)
	}
}

func TestSelectFillCandidateTopRatedTie(t *testing.T) {
	candidates := []fillCandidate{
		{seriesID: "s2", episodeID: "e2", rating: 8.0},
		{seriesID: "s1", episodeID: "e1", rating: 8.0},
		{seriesID: "s3", episodeID: "e3", rating: 7.0},
	}
	// Same rating — tie break by seriesID (lower wins)
	result := selectFillCandidate(candidates, "top_rated", 0)
	if result.seriesID != "s1" {
		t.Error("expected tie to break to s1, got:", result.seriesID)
	}
}

func TestSelectFillCandidateDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candidates := []fillCandidate{
		{seriesID: "s1", episodeID: "e1", rating: 8.0},
		{seriesID: "s2", episodeID: "e2", rating: 7.0},
		{seriesID: "s3", episodeID: "e3", rating: 9.0},
	}
	// Create a random permutation
	shuffled := make([]fillCandidate, len(candidates))
	copy(shuffled, candidates)
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	result1 := selectFillCandidate(shuffled, "top_rated", 0)
	result2 := selectFillCandidate(shuffled, "top_rated", 0)
	if result1.episodeID != result2.episodeID {
		t.Error("selectFillCandidate must be deterministic")
	}
}
