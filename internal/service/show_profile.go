package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/andrew/rotator/internal/repository"
	"github.com/google/uuid"
)

// ShowProfileRules applies episode overrides before season overrides, then the
// profile's default. This keeps allow-list and deny-list profiles predictable.
type ShowProfileRules struct {
	DefaultAllow bool
	Seasons      map[int]bool
	Episodes     map[string]bool
}

func (r ShowProfileRules) Allows(ep repository.Episode) bool {
	if allowed, ok := r.Episodes[ep.ID]; ok {
		return allowed
	}
	if allowed, ok := r.Seasons[ep.SeasonNumber]; ok {
		return allowed
	}
	return r.DefaultAllow
}

func filterAllowedEpisodes(episodes []repository.Episode, rules ShowProfileRules) []repository.Episode {
	allowed := make([]repository.Episode, 0, len(episodes))
	for _, ep := range episodes {
		if rules.Allows(ep) {
			allowed = append(allowed, ep)
		}
	}
	return allowed
}

type ShowProfileResponse struct {
	repository.ShowProfile
	EligibleEpisodes int `json:"eligible_episodes"`
	Assignments      int `json:"assignments"`
}

type ShowProfileDetailResponse struct {
	ShowProfileResponse
	SeasonRules  []repository.ShowProfileSeasonRule  `json:"season_rules"`
	EpisodeRules []repository.ShowProfileEpisodeRule `json:"episode_rules"`
}

type ShowProfileUpdateInput struct {
	Name         string
	DefaultMode  string
	SeasonRules  []repository.ShowProfileSeasonRule
	EpisodeRules []repository.ShowProfileEpisodeRule
}

func (s *Service) ListShowProfiles(ctx context.Context, seriesID string) ([]ShowProfileResponse, error) {
	if _, err := s.seriesRepo.GetByID(ctx, seriesID); err != nil {
		return nil, err
	}
	profiles, err := s.showProfileRepo.ListBySeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	result := make([]ShowProfileResponse, 0, len(profiles))
	for _, profile := range profiles {
		detail, err := s.showProfileDetail(ctx, &profile)
		if err != nil {
			return nil, err
		}
		result = append(result, detail.ShowProfileResponse)
	}
	return result, nil
}

func (s *Service) CreateShowProfile(ctx context.Context, seriesID, name, defaultMode string) (*ShowProfileDetailResponse, error) {
	if _, err := s.seriesRepo.GetByID(ctx, seriesID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if defaultMode != "allow" && defaultMode != "deny" {
		return nil, fmt.Errorf("default mode must be allow or deny")
	}
	p, err := s.showProfileRepo.Create(ctx, &repository.ShowProfile{ID: uuid.NewString(), SeriesID: seriesID, Name: strings.TrimSpace(name), DefaultMode: defaultMode})
	if err != nil {
		return nil, err
	}
	return s.showProfileDetail(ctx, p)
}

func (s *Service) GetShowProfile(ctx context.Context, seriesID, profileID string) (*ShowProfileDetailResponse, error) {
	p, err := s.profileForSeries(ctx, seriesID, profileID)
	if err != nil {
		return nil, err
	}
	return s.showProfileDetail(ctx, p)
}

func (s *Service) UpdateShowProfile(ctx context.Context, seriesID, profileID string, input ShowProfileUpdateInput) (*ShowProfileDetailResponse, error) {
	p, err := s.profileForSeries(ctx, seriesID, profileID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	if input.DefaultMode != "allow" && input.DefaultMode != "deny" {
		return nil, fmt.Errorf("default mode must be allow or deny")
	}
	if err := s.validateProfileRules(ctx, seriesID, input.SeasonRules, input.EpisodeRules); err != nil {
		return nil, err
	}
	p.Name, p.DefaultMode = strings.TrimSpace(input.Name), input.DefaultMode
	if err := s.showProfileRepo.Update(ctx, p); err != nil {
		return nil, err
	}
	if err := s.showProfileRepo.ReplaceRules(ctx, profileID, input.SeasonRules, input.EpisodeRules); err != nil {
		return nil, err
	}
	// Existing candidates were selected under the old rules. Refill will replace
	// skipped entries; watching entries are deliberately left alone.
	if err := s.skipExcludedQueuedEpisodes(ctx, profileID); err != nil {
		return nil, err
	}
	return s.GetShowProfile(ctx, seriesID, profileID)
}

func (s *Service) DeleteShowProfile(ctx context.Context, seriesID, profileID string) error {
	p, err := s.profileForSeries(ctx, seriesID, profileID)
	if err != nil {
		return err
	}
	if p.IsDefault {
		return fmt.Errorf("the default profile cannot be deleted")
	}
	defaultProfile, err := s.showProfileRepo.GetDefaultForSeries(ctx, seriesID)
	if err != nil || defaultProfile == nil {
		return fmt.Errorf("get default show profile: %w", err)
	}
	if _, err := s.showProfileRepo.ReassignMemberships(ctx, profileID, defaultProfile.ID); err != nil {
		return err
	}
	return s.showProfileRepo.Delete(ctx, profileID)
}

func (s *Service) SetDefaultShowProfile(ctx context.Context, seriesID, profileID string) error {
	if _, err := s.profileForSeries(ctx, seriesID, profileID); err != nil {
		return err
	}
	return s.showProfileRepo.SetDefault(ctx, seriesID, profileID)
}

func (s *Service) PreviewShowProfile(ctx context.Context, seriesID, profileID string) (*ShowProfileResponse, error) {
	p, err := s.profileForSeries(ctx, seriesID, profileID)
	if err != nil {
		return nil, err
	}
	detail, err := s.showProfileDetail(ctx, p)
	if err != nil {
		return nil, err
	}
	return &detail.ShowProfileResponse, nil
}

func (s *Service) profileForSeries(ctx context.Context, seriesID, profileID string) (*repository.ShowProfile, error) {
	p, err := s.showProfileRepo.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if p.SeriesID != seriesID {
		return nil, fmt.Errorf("show profile does not belong to series")
	}
	return p, nil
}

func (s *Service) showProfileDetail(ctx context.Context, p *repository.ShowProfile) (*ShowProfileDetailResponse, error) {
	rules, err := s.profileRules(ctx, &p.ID)
	if err != nil {
		return nil, err
	}
	episodes, err := s.episodeRepo.ListBySeries(ctx, p.SeriesID)
	if err != nil {
		return nil, err
	}
	assignments, err := s.showProfileRepo.CountAssignments(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	seasonRules, err := s.showProfileRepo.ListSeasonRules(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	episodeRules, err := s.showProfileRepo.ListEpisodeRules(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return &ShowProfileDetailResponse{ShowProfileResponse: ShowProfileResponse{ShowProfile: *p, EligibleEpisodes: len(filterAllowedEpisodes(episodes, rules)), Assignments: assignments}, SeasonRules: seasonRules, EpisodeRules: episodeRules}, nil
}

func (s *Service) profileRules(ctx context.Context, profileID *string) (ShowProfileRules, error) {
	rules := ShowProfileRules{DefaultAllow: true, Seasons: make(map[int]bool), Episodes: make(map[string]bool)}
	if profileID == nil || *profileID == "" {
		return rules, nil
	}
	p, err := s.showProfileRepo.GetByID(ctx, *profileID)
	if err != nil {
		return rules, err
	}
	rules.DefaultAllow = p.DefaultMode == "allow"
	seasonRules, err := s.showProfileRepo.ListSeasonRules(ctx, p.ID)
	if err != nil {
		return rules, err
	}
	for _, rule := range seasonRules {
		rules.Seasons[rule.SeasonNumber] = rule.Allowed
	}
	episodeRules, err := s.showProfileRepo.ListEpisodeRules(ctx, p.ID)
	if err != nil {
		return rules, err
	}
	for _, rule := range episodeRules {
		rules.Episodes[rule.EpisodeID] = rule.Allowed
	}
	return rules, nil
}

func (s *Service) validateProfileRules(ctx context.Context, seriesID string, seasonRules []repository.ShowProfileSeasonRule, episodeRules []repository.ShowProfileEpisodeRule) error {
	seenSeasons := make(map[int]bool)
	for _, rule := range seasonRules {
		if rule.SeasonNumber < 0 || seenSeasons[rule.SeasonNumber] {
			return fmt.Errorf("invalid duplicate season rule")
		}
		seenSeasons[rule.SeasonNumber] = true
	}
	seenEpisodes := make(map[string]bool)
	for _, rule := range episodeRules {
		if rule.EpisodeID == "" || seenEpisodes[rule.EpisodeID] {
			return fmt.Errorf("invalid duplicate episode rule")
		}
		seenEpisodes[rule.EpisodeID] = true
		ep, err := s.episodeRepo.GetByID(ctx, rule.EpisodeID)
		if err != nil || ep.SeriesID != seriesID {
			return fmt.Errorf("episode rule does not belong to series")
		}
	}
	return nil
}

func (s *Service) skipExcludedQueuedEpisodes(ctx context.Context, profileID string) error {
	rules, err := s.profileRules(ctx, &profileID)
	if err != nil {
		return err
	}
	playlists, err := s.playlistRepo.List(ctx)
	if err != nil {
		return err
	}
	for _, playlist := range playlists {
		members, err := s.playlistRepo.ListSeries(ctx, playlist.ID)
		if err != nil {
			return err
		}
		for _, member := range members {
			if member.ShowProfileID == nil || *member.ShowProfileID != profileID {
				continue
			}
			if member.Mode == "serial" {
				if err := s.moveExcludedCursor(ctx, member, rules); err != nil {
					return err
				}
			}
			items, err := s.playlistRepo.ListQueueItems(ctx, playlist.ID)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.SeriesID != member.SeriesID || (item.Status != "pending" && item.Status != "pushed") {
					continue
				}
				ep, err := s.episodeRepo.GetByID(ctx, item.EpisodeID)
				if err == nil && !rules.Allows(*ep) {
					_ = s.playlistRepo.UpdateItemStatus(ctx, item.ID, "skipped")
				}
			}
		}
	}
	return nil
}

func (s *Service) moveExcludedCursor(ctx context.Context, member repository.PlaylistSeries, rules ShowProfileRules) error {
	progress, err := s.playlistRepo.GetProgress(ctx, member.ID)
	if err != nil || progress == nil || progress.NextEpisodeID == nil {
		return err
	}
	current, err := s.episodeRepo.GetByID(ctx, *progress.NextEpisodeID)
	if err != nil || rules.Allows(*current) {
		return err
	}
	episodes, err := s.episodeRepo.ListBySeries(ctx, member.SeriesID)
	if err != nil {
		return err
	}
	next, ok := firstAllowedUnqueuedEpisodeAtCursor(episodes, *progress.NextEpisodeID, progress.NextPosition, nil, rules)
	updated := &repository.PlaylistProgress{ID: uuid.NewString(), PlaylistSeriesID: member.ID}
	if ok {
		updated.NextEpisodeID = &next.ID
		updated.NextPosition = &next.AbsoluteOrder
	}
	return s.playlistRepo.UpsertProgress(ctx, updated)
}

func profileSeasonNumbers(episodes []repository.Episode) []int {
	seen := make(map[int]bool)
	for _, ep := range episodes {
		seen[ep.SeasonNumber] = true
	}
	seasons := make([]int, 0, len(seen))
	for season := range seen {
		seasons = append(seasons, season)
	}
	sort.Ints(seasons)
	return seasons
}
