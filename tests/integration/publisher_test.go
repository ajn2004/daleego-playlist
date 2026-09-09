package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrew/rotator/internal/media/plex"
)

func TestProductionPublisherRejectsAcceptedShortWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<MediaContainer machineIdentifier="test-server"/>`)
		case "/playlists":
			fmt.Fprint(w, `<MediaContainer><Playlist ratingKey="1" title="TV Rotation"/></MediaContainer>`)
		case "/playlists/1/items":
			fmt.Fprint(w, `<MediaContainer size="8" totalSize="8"><Video ratingKey="1"/><Video ratingKey="2"/><Video ratingKey="3"/><Video ratingKey="4"/><Video ratingKey="5"/><Video ratingKey="6"/><Video ratingKey="7"/><Video ratingKey="8"/></MediaContainer>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := plex.NewClient(server.URL, "test-token", 5*time.Second).UpsertPlaylist(context.Background(), nil, "TV Rotation", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"})
	var mismatch *plex.PublicationMismatchError
	if !errors.As(err, &mismatch) || len(mismatch.Missing) != 2 {
		t.Fatalf("error = %v, want two missing IDs", err)
	}
}

func TestProductionPlaylistReaderCollectsAllPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Query().Get("offset") == "0" {
			fmt.Fprint(w, `<MediaContainer offset="0" size="8" totalSize="10"><Video ratingKey="1"/><Video ratingKey="2"/><Video ratingKey="3"/><Video ratingKey="4"/><Video ratingKey="5"/><Video ratingKey="6"/><Video ratingKey="7"/><Video ratingKey="8"/></MediaContainer>`)
			return
		}
		fmt.Fprint(w, `<MediaContainer offset="8" size="2" totalSize="10"><Video ratingKey="9"/><Video ratingKey="10"/></MediaContainer>`)
	}))
	defer server.Close()
	items, err := plex.NewClient(server.URL, "test-token", 5*time.Second).ListPlaylistItems(context.Background(), "1")
	if err != nil || len(items) != 10 || strings.TrimSpace(items[9].EpisodeID) != "10" {
		t.Fatalf("items=%d last=%v err=%v", len(items), items, err)
	}
}
