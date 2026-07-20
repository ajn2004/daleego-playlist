package service

import (
	"testing"

	"github.com/andrew/rotator/internal/repository"
)

func TestShowProfileRulesPrecedence(t *testing.T) {
	rules := ShowProfileRules{
		DefaultAllow: false,
		Seasons:      map[int]bool{2: true, 3: false},
		Episodes:     map[string]bool{"override-allow": true, "override-deny": false},
	}
	tests := []struct {
		name string
		ep   repository.Episode
		want bool
	}{
		{"season allow", repository.Episode{ID: "s2", SeasonNumber: 2}, true},
		{"season deny", repository.Episode{ID: "s3", SeasonNumber: 3}, false},
		{"default deny", repository.Episode{ID: "s1", SeasonNumber: 1}, false},
		{"episode allow overrides season", repository.Episode{ID: "override-allow", SeasonNumber: 3}, true},
		{"episode deny overrides season", repository.Episode{ID: "override-deny", SeasonNumber: 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rules.Allows(tt.ep); got != tt.want {
				t.Fatalf("Allows(%#v) = %v, want %v", tt.ep, got, tt.want)
			}
		})
	}
}

func TestFilterAllowedEpisodes(t *testing.T) {
	episodes := []repository.Episode{{ID: "s1", SeasonNumber: 1}, {ID: "s2", SeasonNumber: 2}, {ID: "s3", SeasonNumber: 3}}
	allowed := filterAllowedEpisodes(episodes, ShowProfileRules{DefaultAllow: false, Seasons: map[int]bool{2: true, 3: true}, Episodes: map[string]bool{"s3": false}})
	if len(allowed) != 1 || allowed[0].ID != "s2" {
		t.Fatalf("allowed = %#v, want only s2", allowed)
	}
}

func TestFirstAllowedUnqueuedEpisodeAtCursorSkipsExcluded(t *testing.T) {
	position := 2
	episodes := []repository.Episode{{ID: "e1", AbsoluteOrder: 1}, {ID: "e2", AbsoluteOrder: 2}, {ID: "e3", AbsoluteOrder: 3}}
	rules := ShowProfileRules{DefaultAllow: true, Episodes: map[string]bool{"e2": false}}
	ep, ok := firstAllowedUnqueuedEpisodeAtCursor(episodes, "e2", &position, nil, rules)
	if !ok || ep.ID != "e3" {
		t.Fatalf("got %#v, %v; want e3, true", ep, ok)
	}
}
