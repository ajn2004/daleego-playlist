package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetPlaylistSeriesRejectsInvalidMembershipInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "missing series ID",
			body: `{"series":[{"series_id":"","mode":"serial"}]}`,
			code: "missing_series_id",
		},
		{
			name: "duplicate series ID",
			body: `{"series":[{"series_id":"series-1","mode":"serial"},{"series_id":"series-1","mode":"non_serial"}]}`,
			code: "duplicate_series",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/playlist-1/series", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			(&Server{}).handleSetPlaylistSeries(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"`+tt.code+`"`)) {
				t.Fatalf("response = %s, want error code %q", rec.Body.String(), tt.code)
			}
		})
	}
}
