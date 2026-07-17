package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTauriCORSAllowsTauriOrigins(t *testing.T) {
	t.Parallel()

	handler := tauriCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/status", nil)
	req.Header.Set("Origin", "tauri://localhost")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "tauri://localhost" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want tauri origin", got)
	}
}

func TestTauriCORSDoesNotAllowOtherOrigins(t *testing.T) {
	t.Parallel()

	handler := tauriCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}
