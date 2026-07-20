package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/andrew/rotator/internal/repository"
	"github.com/andrew/rotator/internal/service"
)

func (s *Server) handleListShowProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.svc.ListShowProfiles(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"show_profiles": profiles})
}

func (s *Server) handleCreateShowProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		DefaultMode string `json:"default_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	profile, err := s.svc.CreateShowProfile(r.Context(), r.PathValue("id"), req.Name, req.DefaultMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleGetShowProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.svc.GetShowProfile(r.Context(), r.PathValue("id"), r.PathValue("profileID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleUpdateShowProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string                              `json:"name"`
		DefaultMode  string                              `json:"default_mode"`
		SeasonRules  []repository.ShowProfileSeasonRule  `json:"season_rules"`
		EpisodeRules []repository.ShowProfileEpisodeRule `json:"episode_rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	profile, err := s.svc.UpdateShowProfile(r.Context(), r.PathValue("id"), r.PathValue("profileID"), service.ShowProfileUpdateInput{Name: req.Name, DefaultMode: req.DefaultMode, SeasonRules: req.SeasonRules, EpisodeRules: req.EpisodeRules})
	if err != nil {
		writeError(w, http.StatusBadRequest, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleDeleteShowProfile(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteShowProfile(r.Context(), r.PathValue("id"), r.PathValue("profileID")); err != nil {
		writeError(w, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSetDefaultShowProfile(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.SetDefaultShowProfile(r.Context(), r.PathValue("id"), r.PathValue("profileID")); err != nil {
		writeError(w, http.StatusBadRequest, "set_default_failed", err.Error())
		return
	}
	profile, _ := s.svc.GetShowProfile(r.Context(), r.PathValue("id"), r.PathValue("profileID"))
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handlePreviewShowProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.svc.PreviewShowProfile(r.Context(), r.PathValue("id"), r.PathValue("profileID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func validProfileMode(mode string) bool {
	return strings.TrimSpace(mode) == "allow" || strings.TrimSpace(mode) == "deny"
}
