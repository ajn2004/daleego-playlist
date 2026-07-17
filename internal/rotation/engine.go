package rotation

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

type Slot struct {
	Name      string `json:"name"`
	RankBy    string `json:"rank_by"`
	Direction string `json:"direction"`
	PoolSize  int    `json:"pool_size"`
	Selection string `json:"selection"`
}

type Scoring struct {
	EpisodeWeight float64 `json:"episode_weight"`
	ArcWeight     float64 `json:"arc_weight"`
	WindowSize    int     `json:"window_size"`
	WindowDecay   float64 `json:"window_decay"`
}

type Constraints struct {
	DifferentSeriesPerRotation bool `json:"different_series_per_rotation"`
	IncludeSpecials            bool `json:"include_specials"`
	SessionBudgetMinutes       int  `json:"session_budget_minutes"`
}

type Policy struct {
	Slots       []Slot      `json:"slots"`
	Scoring     Scoring     `json:"scoring"`
	Constraints Constraints `json:"constraints"`
}

func (p *Policy) Validate() error {
	if len(p.Slots) == 0 {
		return fmt.Errorf("at least one slot required")
	}
	for _, s := range p.Slots {
		if s.Name == "" {
			return fmt.Errorf("slot name required")
		}
		if s.RankBy != "arc_rating" && s.RankBy != "episode_rating" && s.RankBy != "combined_score" {
			return fmt.Errorf("invalid rank_by: %s", s.RankBy)
		}
		if s.Direction != "ascending" && s.Direction != "descending" {
			return fmt.Errorf("invalid direction: %s", s.Direction)
		}
		if s.PoolSize < 1 {
			return fmt.Errorf("pool_size must be >= 1")
		}
		if s.Selection != "random" && s.Selection != "top" {
			return fmt.Errorf("invalid selection: %s", s.Selection)
		}
	}
	if p.Scoring.EpisodeWeight < 0 || p.Scoring.ArcWeight < 0 {
		return fmt.Errorf("weights must be non-negative")
	}
	if p.Scoring.WindowSize < 1 {
		return fmt.Errorf("window_size must be >= 1")
	}
	if p.Scoring.WindowDecay <= 0 || p.Scoring.WindowDecay > 1 {
		return fmt.Errorf("window_decay must be in (0, 1]")
	}
	if p.Constraints.SessionBudgetMinutes < 0 {
		return fmt.Errorf("session_budget_minutes must be >= 0")
	}
	return nil
}

type Candidate struct {
	SeriesID      string
	EpisodeID     string
	SeriesTitle   string
	EpisodeTitle  string
	SeasonNumber  int
	EpisodeNumber int
	Duration      int
	EpisodeRating float64
	ArcRating     float64
	CombinedScore float64
	WindowRatings []float64
	LastWatchedAt *time.Time
}

type RotationItem struct {
	Position      int                    `json:"position"`
	SeriesID      string                 `json:"series_id"`
	EpisodeID     string                 `json:"episode_id"`
	SeriesTitle   string                 `json:"series_title"`
	EpisodeTitle  string                 `json:"episode_title"`
	SeasonNumber  int                    `json:"season_number"`
	EpisodeNumber int                    `json:"episode_number"`
	SlotKind      string                 `json:"slot_kind"`
	Score         float64                `json:"score"`
	ScoreDetails  map[string]interface{} `json:"score_details"`
}

type RotationResult struct {
	Items []RotationItem `json:"items"`
	Seed  int64          `json:"seed"`
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Generate(candidates []Candidate, policy *Policy, seed int64) (*RotationResult, error) {
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	rng := rand.New(rand.NewSource(seed))

	eligible := filterCandidates(candidates, policy)
	scored := scoreCandidates(eligible, &policy.Scoring)

	usedSeries := make(map[string]bool)
	var items []RotationItem
	budgetSec := policy.Constraints.SessionBudgetMinutes * 60

	for _, slot := range policy.Slots {
		pool := rankPool(scored, slot, usedSeries)

		if len(pool) == 0 {
			continue
		}

		var selected Candidate
		switch slot.Selection {
		case "random":
			selected = pool[rng.Intn(len(pool))]
		case "top":
			selected = pool[0]
		}

		duration := selected.Duration
		if budgetSec > 0 && duration > budgetSec {
			// Try to find a shorter candidate
			shortPool := rankPool(scored, slot, usedSeries)
			found := false
			for _, c := range shortPool {
				if c.Duration <= budgetSec && c.EpisodeID != selected.EpisodeID {
					selected = c
					duration = c.Duration
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		usedSeries[selected.SeriesID] = true
		if budgetSec > 0 {
			budgetSec -= duration
		}

		details := map[string]interface{}{
			"episode_rating": selected.EpisodeRating,
			"arc_rating":     selected.ArcRating,
			"episode_weight": policy.Scoring.EpisodeWeight,
			"arc_weight":     policy.Scoring.ArcWeight,
			"combined_score": selected.CombinedScore,
			"window":         selected.WindowRatings,
		}

		items = append(items, RotationItem{
			Position:      len(items) + 1,
			SeriesID:      selected.SeriesID,
			EpisodeID:     selected.EpisodeID,
			SeriesTitle:   selected.SeriesTitle,
			EpisodeTitle:  selected.EpisodeTitle,
			SeasonNumber:  selected.SeasonNumber,
			EpisodeNumber: selected.EpisodeNumber,
			SlotKind:      slot.Name,
			Score:         selected.CombinedScore,
			ScoreDetails:  details,
		})
	}

	return &RotationResult{Items: items, Seed: seed}, nil
}

func filterCandidates(candidates []Candidate, policy *Policy) []Candidate {
	var filtered []Candidate
	for _, c := range candidates {
		if !policy.Constraints.IncludeSpecials && c.SeasonNumber == 0 {
			continue
		}
		if c.Duration <= 0 {
			continue
		}
		if policy.Constraints.SessionBudgetMinutes > 0 && c.Duration > policy.Constraints.SessionBudgetMinutes*60 {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

func scoreCandidates(candidates []Candidate, scoring *Scoring) []Candidate {
	scored := make([]Candidate, len(candidates))
	for i, c := range candidates {
		window := c.WindowRatings
		if window == nil {
			window = []float64{}
		}
		arcRating := WeightedArcRating(window, scoring.WindowDecay)
		combined := c.EpisodeRating*scoring.EpisodeWeight + arcRating*scoring.ArcWeight
		scored[i] = c
		scored[i].ArcRating = arcRating
		scored[i].CombinedScore = combined
		scored[i].WindowRatings = window
	}
	return scored
}

func WeightedArcRating(ratings []float64, decay float64) float64 {
	if len(ratings) == 0 {
		return 0
	}
	var sum, totalWeight float64
	for offset, rating := range ratings {
		weight := math.Pow(decay, float64(offset))
		sum += rating * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	return sum / totalWeight
}

func rankPool(candidates []Candidate, slot Slot, usedSeries map[string]bool) []Candidate {
	var pool []Candidate
	for _, c := range candidates {
		if usedSeries[c.SeriesID] {
			continue
		}
		pool = append(pool, c)
	}

	var rankField func(c Candidate) float64
	switch slot.RankBy {
	case "arc_rating":
		rankField = func(c Candidate) float64 { return c.ArcRating }
	case "episode_rating":
		rankField = func(c Candidate) float64 { return c.EpisodeRating }
	case "combined_score":
		rankField = func(c Candidate) float64 { return c.CombinedScore }
	default:
		rankField = func(c Candidate) float64 { return c.CombinedScore }
	}

	for i := 0; i < len(pool); i++ {
		for j := i + 1; j < len(pool); j++ {
			less := rankField(pool[i]) < rankField(pool[j])
			if slot.Direction == "descending" {
				less = rankField(pool[j]) < rankField(pool[i])
			}
			if less {
				pool[i], pool[j] = pool[j], pool[i]
			}
		}
	}

	if slot.PoolSize > 0 && len(pool) > slot.PoolSize {
		pool = pool[:slot.PoolSize]
	}

	return pool
}

// ScoreDetailsJSON returns the JSON representation of score details
func ScoreDetailsJSON(details map[string]interface{}) string {
	b, _ := json.Marshal(details)
	return string(b)
}
