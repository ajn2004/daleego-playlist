package rotation

import "testing"

func TestWeightedArcRating(t *testing.T) {
	tests := []struct {
		name     string
		ratings  []float64
		decay    float64
		expected float64
	}{
		{
			name:     "empty",
			ratings:  nil,
			decay:    0.72,
			expected: 0,
		},
		{
			name:     "single rating",
			ratings:  []float64{8.0},
			decay:    0.72,
			expected: 8.0,
		},
		{
			name:     "equal ratings",
			ratings:  []float64{7.0, 7.0, 7.0},
			decay:    0.72,
			expected: 7.0,
		},
		{
			name:     "decay weights recent higher",
			ratings:  []float64{5.0, 9.0},
			decay:    0.72,
			expected: 6.674419, // weighted average
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeightedArcRating(tt.ratings, tt.decay)
			if got != tt.expected {
				// Allow for floating point tolerance
				diff := got - tt.expected
				if diff < 0 {
					diff = -diff
				}
				if diff > 0.01 {
					t.Errorf("got %f, want %f", got, tt.expected)
				}
			}
		})
	}
}

func TestPolicyValidate(t *testing.T) {
	valid := &Policy{
		Slots: []Slot{
			{Name: "high", RankBy: "arc_rating", Direction: "descending", PoolSize: 5, Selection: "random"},
		},
		Scoring: Scoring{
			EpisodeWeight: 0.4,
			ArcWeight:     0.6,
			WindowSize:    5,
			WindowDecay:   0.72,
		},
		Constraints: Constraints{
			DifferentSeriesPerRotation: true,
			SessionBudgetMinutes:       120,
		},
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("valid policy should pass: %v", err)
	}

	invalid := &Policy{
		Slots: []Slot{},
	}
	if err := invalid.Validate(); err == nil {
		t.Error("empty slots should be invalid")
	}
}

func TestGenerateDeterministic(t *testing.T) {
	candidates := []Candidate{
		{SeriesID: "1", EpisodeID: "e1", SeriesTitle: "A", EpisodeTitle: "Ep1", EpisodeRating: 8.0, Duration: 1800, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{8.0, 8.5, 9.0}},
		{SeriesID: "2", EpisodeID: "e2", SeriesTitle: "B", EpisodeTitle: "Ep2", EpisodeRating: 7.0, Duration: 1500, SeasonNumber: 2, EpisodeNumber: 3, WindowRatings: []float64{7.0, 7.5, 8.0}},
		{SeriesID: "3", EpisodeID: "e3", SeriesTitle: "C", EpisodeTitle: "Ep3", EpisodeRating: 9.0, Duration: 2400, SeasonNumber: 1, EpisodeNumber: 5, WindowRatings: []float64{9.0, 8.5, 8.0}},
	}

	policy := &Policy{
		Slots: []Slot{
			{Name: "high-1", RankBy: "arc_rating", Direction: "descending", PoolSize: 3, Selection: "top"},
		},
		Scoring: Scoring{
			EpisodeWeight: 0.4,
			ArcWeight:     0.6,
			WindowSize:    5,
			WindowDecay:   0.72,
		},
		Constraints: Constraints{
			DifferentSeriesPerRotation: true,
			SessionBudgetMinutes:       0,
		},
	}

	engine := NewEngine()
	result1, err := engine.Generate(candidates, policy, 42)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	result2, err := engine.Generate(candidates, policy, 42)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if len(result1.Items) != len(result2.Items) {
		t.Fatal("different item counts for same seed")
	}
	for i, item := range result1.Items {
		if item.EpisodeID != result2.Items[i].EpisodeID {
			t.Errorf("item %d differs between runs", i)
		}
	}
}

func TestDistinctSeries(t *testing.T) {
	candidates := []Candidate{
		{SeriesID: "1", EpisodeID: "e1", SeriesTitle: "A", EpisodeRating: 8.0, Duration: 1800, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{8.0}},
		{SeriesID: "1", EpisodeID: "e2", SeriesTitle: "A", EpisodeRating: 8.0, Duration: 1800, SeasonNumber: 1, EpisodeNumber: 2, WindowRatings: []float64{8.0}},
		{SeriesID: "2", EpisodeID: "e3", SeriesTitle: "B", EpisodeRating: 7.0, Duration: 1500, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{7.0}},
	}

	policy := &Policy{
		Slots: []Slot{
			{Name: "s1", RankBy: "episode_rating", Direction: "descending", PoolSize: 3, Selection: "top"},
			{Name: "s2", RankBy: "episode_rating", Direction: "descending", PoolSize: 3, Selection: "top"},
		},
		Scoring: Scoring{
			EpisodeWeight: 1.0,
			ArcWeight:     0.0,
			WindowSize:    1,
			WindowDecay:   1.0,
		},
		Constraints: Constraints{
			DifferentSeriesPerRotation: true,
			SessionBudgetMinutes:       0,
		},
	}

	engine := NewEngine()
	result, err := engine.Generate(candidates, policy, 1)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	if result.Items[0].SeriesID == result.Items[1].SeriesID {
		t.Error("same series appeared twice in rotation")
	}
}

func TestDurationBudget(t *testing.T) {
	candidates := []Candidate{
		{SeriesID: "1", EpisodeID: "e1", SeriesTitle: "A", EpisodeRating: 8.0, Duration: 3600, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{8.0}},
		{SeriesID: "2", EpisodeID: "e2", SeriesTitle: "B", EpisodeRating: 7.0, Duration: 1200, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{7.0}},
		{SeriesID: "3", EpisodeID: "e3", SeriesTitle: "C", EpisodeRating: 6.0, Duration: 600, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{6.0}},
	}

	policy := &Policy{
		Slots: []Slot{
			{Name: "s1", RankBy: "episode_rating", Direction: "descending", PoolSize: 3, Selection: "top"},
			{Name: "s2", RankBy: "episode_rating", Direction: "descending", PoolSize: 3, Selection: "top"},
		},
		Scoring: Scoring{
			EpisodeWeight: 1.0,
			ArcWeight:     0.0,
			WindowSize:    1,
			WindowDecay:   1.0,
		},
		Constraints: Constraints{
			DifferentSeriesPerRotation: true,
			SessionBudgetMinutes:       30, // 1800 seconds
		},
	}

	engine := NewEngine()
	result, err := engine.Generate(candidates, policy, 1)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if len(result.Items) == 0 {
		t.Fatal("expected at least one item")
	}

	// With 30 minute budget, only the 20min and 10min episodes fit
	// The 60min episode is excluded by the filter, and only 2 slots exist
	// The first slot selects the highest rated (series 1, 60min - excluded by filter)
	// so it falls back to the next available candidate
	if len(result.Items) == 0 {
		t.Fatal("expected at least one item")
	}
	// All items should fit within the budget
	totalDuration := 0
	for _, item := range result.Items {
		// Find the candidate duration
		for _, c := range candidates {
			if c.EpisodeID == item.EpisodeID {
				totalDuration += c.Duration
			}
		}
	}
	if totalDuration > 30*60 {
		t.Errorf("total duration %d exceeds budget", totalDuration)
	}
}

func TestFilterSpecials(t *testing.T) {
	candidates := []Candidate{
		{SeriesID: "1", EpisodeID: "e1", EpisodeRating: 8.0, Duration: 1800, SeasonNumber: 0, EpisodeNumber: 1, WindowRatings: []float64{8.0}},
		{SeriesID: "2", EpisodeID: "e2", EpisodeRating: 7.0, Duration: 1800, SeasonNumber: 1, EpisodeNumber: 1, WindowRatings: []float64{7.0}},
	}

	policy := &Policy{
		Slots: []Slot{
			{Name: "s1", RankBy: "episode_rating", Direction: "descending", PoolSize: 3, Selection: "top"},
		},
		Scoring: Scoring{
			EpisodeWeight: 1.0,
			ArcWeight:     0.0,
			WindowSize:    1,
			WindowDecay:   1.0,
		},
		Constraints: Constraints{
			IncludeSpecials: false,
		},
	}

	engine := NewEngine()
	result, err := engine.Generate(candidates, policy, 1)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}

	if result.Items[0].EpisodeID != "e2" {
		t.Error("should have selected the non-special episode")
	}
}
