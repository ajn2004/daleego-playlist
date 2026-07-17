package server

import (
	"encoding/json"
	"net/http"

	"github.com/andrew/rotator/internal/rotation"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.pool.Ping(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"plex":   s.cfg.PlexURL,
	})
}

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.svc.ListServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"media_servers": servers})
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if req.URL == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "url and token are required")
		return
	}
	srv, err := s.svc.CreateServer(r.Context(), req.URL, req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, srv)
}

func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.TestServer(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, "test_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListSeries(w http.ResponseWriter, r *http.Request) {
	series, err := s.svc.ListSeries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"series": series})
}

func (s *Server) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	series, err := s.svc.GetSeries(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, series)
}

func (s *Server) handleUpdateSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Active *bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if req.Active != nil {
		if err := s.svc.SetActive(r.Context(), id, *req.Active); err != nil {
			writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
			return
		}
	}
	series, err := s.svc.GetSeries(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, series)
}

func (s *Server) handleSyncSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.SyncSeries(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReconcileSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.ReconcileSeries(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "reconcile_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.svc.ListProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"rotation_profiles": profiles})
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req rotation.Policy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	profile, err := s.svc.CreateProfile(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req rotation.Policy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	profile, err := s.svc.UpdateProfile(r.Context(), id, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handlePreviewProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	preview, err := s.svc.PreviewProfile(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleCurrentRotation(w http.ResponseWriter, r *http.Request) {
	rotation, err := s.svc.CurrentRotation(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rotation)
}

func (s *Server) handleGenerateRotation(w http.ResponseWriter, r *http.Request) {
	rot, err := s.svc.GenerateRotation(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rot)
}

func (s *Server) handlePublishRotation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.PublishRotation(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "publish_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

func (s *Server) handleRerollRotation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rot, err := s.svc.RerollRotation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reroll_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rot)
}

func (s *Server) handleSyncRotation(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.SyncRotation(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

// --- Playlist Handlers ---

func (s *Server) handleListPlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := s.svc.ListPlaylists(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"playlists": playlists})
}

func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaServerID    string `json:"media_server_id"`
		Name             string `json:"name"`
		PlexPlaylistName string `json:"plex_playlist_name"`
		QueueTargetCount int    `json:"queue_target_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if req.MediaServerID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "media_server_id and name are required")
		return
	}
	if req.PlexPlaylistName == "" {
		req.PlexPlaylistName = req.Name
	}
	if req.QueueTargetCount < 1 {
		req.QueueTargetCount = 10
	}
	playlist, err := s.svc.CreatePlaylist(r.Context(), req.MediaServerID, req.Name, req.PlexPlaylistName, req.QueueTargetCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, playlist)
}

func (s *Server) handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	playlist, err := s.svc.GetPlaylist(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, playlist)
}

func (s *Server) handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name             *string `json:"name"`
		PlexPlaylistName *string `json:"plex_playlist_name"`
		QueueTargetCount *int    `json:"queue_target_count"`
		Enabled          *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}

	existing, err := s.svc.GetPlaylist(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	name := existing.Name
	plexName := existing.PlexPlaylistName
	targetCount := existing.QueueTargetCount
	enabled := existing.Enabled

	if req.Name != nil {
		name = *req.Name
	}
	if req.PlexPlaylistName != nil {
		plexName = *req.PlexPlaylistName
	}
	if req.QueueTargetCount != nil {
		targetCount = *req.QueueTargetCount
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if err := s.svc.UpdatePlaylist(r.Context(), id, name, plexName, targetCount, enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	updated, _ := s.svc.GetPlaylist(r.Context(), id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.DeletePlaylist(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSetPlaylistSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Series []struct {
			SeriesID string `json:"series_id"`
			Mode     string `json:"mode"`
		} `json:"series"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	for _, sr := range req.Series {
		if sr.Mode != "serial" && sr.Mode != "non_serial" {
			writeError(w, http.StatusBadRequest, "invalid_mode", "mode must be serial or non_serial")
			return
		}
	}
	if err := s.svc.SetPlaylistSeries(r.Context(), id, req.Series); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	updated, _ := s.svc.GetPlaylist(r.Context(), id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleSetPlaylistSlots(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Slots []struct {
			SlotType string `json:"slot_type"`
		} `json:"slots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if len(req.Slots) == 0 {
		writeError(w, http.StatusBadRequest, "missing_slots", "at least one slot required")
		return
	}
	slotTypes := make([]string, len(req.Slots))
	for i, s := range req.Slots {
		if s.SlotType != "top_rated" && s.SlotType != "any" && s.SlotType != "lowest_rated" {
			writeError(w, http.StatusBadRequest, "invalid_slot_type", "slot_type must be top_rated, any, or lowest_rated")
			return
		}
		slotTypes[i] = s.SlotType
	}
	if err := s.svc.SetPlaylistSlots(r.Context(), id, slotTypes); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	updated, _ := s.svc.GetPlaylist(r.Context(), id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleFillPlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	queued, err := s.svc.FillPlaylist(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fill_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"queued": queued})
}

func (s *Server) handleClearPlaylist(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.ClearPlaylistQueue(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) handleRefillPlaylist(w http.ResponseWriter, r *http.Request) {
	queued, err := s.svc.RefillPlaylist(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refill_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "refilled", "queued": queued})
}

func (s *Server) handlePublishPlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.PublishPlaylist(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "publish_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

func (s *Server) handleSyncPlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	watched, queued, err := s.svc.SyncPlaylist(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "synced", "watched": watched, "queued": queued})
}

func (s *Server) handleGetPlexPlaylist(w http.ResponseWriter, r *http.Request) {
	playlist, err := s.svc.GetPlexPlaylist(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "get_plex_playlist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, playlist)
}

func (s *Server) handleReplacePlexPlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerEpisodeIDs []string `json:"server_episode_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if len(req.ServerEpisodeIDs) == 0 {
		writeError(w, http.StatusBadRequest, "missing_episodes", "at least one episode is required")
		return
	}
	if err := s.svc.ReplacePlexPlaylist(r.Context(), r.PathValue("id"), req.ServerEpisodeIDs); err != nil {
		writeError(w, http.StatusBadRequest, "replace_plex_playlist_failed", err.Error())
		return
	}
	playlist, err := s.svc.GetPlexPlaylist(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_plex_playlist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, playlist)
}

func (s *Server) handleListPlaylistSeriesEpisodes(w http.ResponseWriter, r *http.Request) {
	playlistID := r.PathValue("playlistID")
	seriesID := r.PathValue("seriesID")
	episodes, err := s.svc.ListPlaylistSeriesEpisodes(r.Context(), playlistID, seriesID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"episodes": episodes})
}

func (s *Server) handleSetPlaylistNextEpisode(w http.ResponseWriter, r *http.Request) {
	playlistID := r.PathValue("playlistID")
	seriesID := r.PathValue("seriesID")
	var req struct {
		EpisodeID string `json:"episode_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body")
		return
	}
	if req.EpisodeID == "" {
		writeError(w, http.StatusBadRequest, "missing_episode_id", "episode_id is required")
		return
	}
	if err := s.svc.SetPlaylistNextEpisode(r.Context(), playlistID, seriesID, req.EpisodeID); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed", err.Error())
		return
	}
	updated, _ := s.svc.GetPlaylist(r.Context(), playlistID)
	writeJSON(w, http.StatusOK, updated)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
