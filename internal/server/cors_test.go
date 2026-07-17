package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTauriCORSAllowsDesktopOrigins(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{
		"tauri://localhost",
		"http://tauri.localhost",
		"http://localhost:8091",
		"http://127.0.0.1:8091",
	} {
		t.Run(origin, func(t *testing.T) {
			handler := tauriCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodOptions, "/api/v1/status", nil)
			req.Header.Set("Origin", origin)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, req)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, origin)
			}
		})
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
