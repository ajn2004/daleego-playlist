package plex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T, fixture string) (*httptest.Server, *Client) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "fixtures", fixture))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", fixture))
		if err != nil {
			t.Fatalf("read fixture %s: %v", fixture, err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(data)
	}))

	client := NewClient(server.URL, "test-token", 5*time.Second)
	return server, client
}

func TestListLibraries(t *testing.T) {
	server, client := testServer(t, "libraries.xml")
	defer server.Close()

	libraries, err := client.ListLibraries(context.Background())
	if err != nil {
		t.Fatalf("ListLibraries failed: %v", err)
	}

	if len(libraries) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(libraries))
	}

	if libraries[0].Type != "show" {
		t.Errorf("expected first library type 'show', got %s", libraries[0].Type)
	}
	if libraries[0].Title != "TV Shows" {
		t.Errorf("expected 'TV Shows', got %s", libraries[0].Title)
	}
	if libraries[0].ID != "1" {
		t.Errorf("expected library ID '1', got %s", libraries[0].ID)
	}
	if libraries[1].ID != "2" {
		t.Errorf("expected library ID '2', got %s", libraries[1].ID)
	}
}

func TestListSeries(t *testing.T) {
	server, client := testServer(t, "series.xml")
	defer server.Close()

	series, err := client.ListSeries(context.Background(), "1")
	if err != nil {
		t.Fatalf("ListSeries failed: %v", err)
	}

	if len(series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(series))
	}
	if series[0].Title != "The Expanse" {
		t.Errorf("expected 'The Expanse', got %s", series[0].Title)
	}
	if series[0].GUID != "plex://show/5d776b2f9f0a9a001f0a0a0a" {
		t.Errorf("expected show GUID, got %q", series[0].GUID)
	}
}

func TestListEpisodes(t *testing.T) {
	server, client := testServer(t, "episodes.xml")
	defer server.Close()

	episodes, err := client.ListEpisodes(context.Background(), "10")
	if err != nil {
		t.Fatalf("ListEpisodes failed: %v", err)
	}

	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(episodes))
	}

	if episodes[0].Title != "Dulcinea" {
		t.Errorf("expected 'Dulcinea', got %s", episodes[0].Title)
	}
	if episodes[0].SeasonNumber != 1 {
		t.Errorf("expected season 1, got %d", episodes[0].SeasonNumber)
	}
	if episodes[0].EpisodeNumber != 1 {
		t.Errorf("expected episode 1, got %d", episodes[0].EpisodeNumber)
	}
	if episodes[0].Rating != 8.2 {
		t.Errorf("expected rating 8.2, got %f", episodes[0].Rating)
	}
	if episodes[1].Rating != 8.1 {
		t.Errorf("expected audience rating fallback 8.1, got %f", episodes[1].Rating)
	}
	if episodes[0].Duration != 2700 {
		t.Errorf("expected duration 2700, got %d", episodes[0].Duration)
	}
	if episodes[0].AirDate != "" {
		t.Errorf("expected empty air date when originallyAvailableAt is absent, got %q", episodes[0].AirDate)
	}
}

func TestUpsertPlaylistUsesServerMetadataURI(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<MediaContainer machineIdentifier="server-id" />`))
		case "/playlists":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			want := "server://server-id/com.plexapp.plugins.library/library/metadata/100,101"
			if got := r.URL.Query().Get("uri"); got != want {
				t.Errorf("playlist URI = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<MediaContainer><Playlist ratingKey="500" title="Rotation" /></MediaContainer>`))
		case "/playlists/500/items":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<MediaContainer size="2"><Video ratingKey="100"/><Video ratingKey="101"/></MediaContainer>`))
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 5*time.Second)
	playlist, err := client.UpsertPlaylist(context.Background(), nil, "Rotation", []string{"100", "101"})
	if err != nil {
		t.Fatalf("UpsertPlaylist failed: %v", err)
	}
	if playlist.ID != "500" {
		t.Errorf("playlist ID = %q, want 500", playlist.ID)
	}
	if requests != 3 {
		t.Errorf("requests = %d, want 3", requests)
	}
}

func TestUpsertPlaylistUpdateUsesServerMetadataURI(t *testing.T) {
	updated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/playlists/500/items":
			switch r.Method {
			case http.MethodGet:
				w.Header().Set("Content-Type", "application/xml")
				if updated {
					_, _ = w.Write([]byte(`<MediaContainer size="2"><Video ratingKey="100"/><Video ratingKey="101"/></MediaContainer>`))
				} else {
					_, _ = w.Write([]byte(`<MediaContainer size="2"><Video ratingKey="99"/><Video ratingKey="100"/></MediaContainer>`))
				}
			case http.MethodDelete:
				want := "server://server-id/com.plexapp.plugins.library/library/metadata/99,100"
				if got := r.URL.Query().Get("uri"); got != want {
					t.Errorf("clear URI = %q, want %q", got, want)
				}
				w.WriteHeader(http.StatusNoContent)
			case http.MethodPut:
				want := "server://server-id/com.plexapp.plugins.library/library/metadata/100,101"
				if got := r.URL.Query().Get("uri"); got != want {
					t.Errorf("playlist URI = %q, want %q", got, want)
				}
				updated = true
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected method %s", r.Method)
			}
		case "/":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<MediaContainer machineIdentifier="server-id" />`))
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 5*time.Second)
	playlistID := "500"
	if _, err := client.UpsertPlaylist(context.Background(), &playlistID, "Rotation", []string{"100", "101"}); err != nil {
		t.Fatalf("UpsertPlaylist failed: %v", err)
	}
}

func TestGetEpisodeProgress(t *testing.T) {
	server, client := testServer(t, "episode_progress.xml")
	defer server.Close()

	progress, err := client.GetEpisodeProgress(context.Background(), []string{"100"})
	if err != nil {
		t.Fatalf("GetEpisodeProgress failed: %v", err)
	}

	if len(progress) != 1 {
		t.Fatalf("expected 1 progress, got %d", len(progress))
	}

	if progress[0].Viewed {
		t.Error("Plex client should not infer a queue-specific viewed event")
	}
	if progress[0].ViewCount != 1 {
		t.Errorf("expected view count 1, got %d", progress[0].ViewCount)
	}
	if progress[0].LastViewedAt != 1700000000 {
		t.Errorf("expected last viewed timestamp 1700000000, got %d", progress[0].LastViewedAt)
	}
}

func TestGetEpisodeProgressDetectsWatching(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<MediaContainer size="1"><Video ratingKey="100" viewCount="0" viewOffset="12345"/></MediaContainer>`))
	}))
	defer server.Close()

	progress, err := NewClient(server.URL, "test-token", 5*time.Second).GetEpisodeProgress(context.Background(), []string{"100"})
	if err != nil {
		t.Fatalf("GetEpisodeProgress failed: %v", err)
	}
	if len(progress) != 1 || !progress[0].Watching || progress[0].Viewed || progress[0].ViewOffset != 12345 {
		t.Errorf("progress = %+v, want unwatched item with active playback offset", progress)
	}
}

func TestClearPlaylistItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/playlists/500/items":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<MediaContainer size="1"><Video ratingKey="100"/></MediaContainer>`))
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<MediaContainer machineIdentifier="server-id"/>`))
		case r.Method == http.MethodDelete && r.URL.Path == "/playlists/500/items":
			if got, want := r.URL.Query().Get("uri"), "server://server-id/com.plexapp.plugins.library/library/metadata/100"; got != want {
				t.Errorf("uri = %q, want %q", got, want)
			}
			if r.URL.Query().Get("X-Plex-Token") != "test-token" {
				t.Error("missing Plex token")
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	if err := NewClient(server.URL, "test-token", 5*time.Second).ClearPlaylistItems(context.Background(), "500"); err != nil {
		t.Fatalf("ClearPlaylistItems failed: %v", err)
	}
}

func TestListPlaylistItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlists/500/items" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<MediaContainer size="2"><Video ratingKey="100" grandparentTitle="The Expanse" title="Dulcinea" parentIndex="1" index="1"/><Video ratingKey="101" grandparentTitle="The Expanse" title="The Big Empty" parentIndex="1" index="2"/></MediaContainer>`))
	}))
	defer server.Close()

	items, err := NewClient(server.URL, "test-token", 5*time.Second).ListPlaylistItems(context.Background(), "500")
	if err != nil {
		t.Fatalf("ListPlaylistItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].EpisodeID != "100" || items[0].SeriesTitle != "The Expanse" || items[0].EpisodeNumber != 1 {
		t.Errorf("first item = %+v", items[0])
	}
}

func TestListPlaylistItemsCollectsPaginatedResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.Header.Get("X-Plex-Container-Start")
		if r.URL.Query().Get("offset") != "" || r.URL.Query().Get("size") != "" {
			t.Errorf("pagination must use Plex headers, got query %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/xml")
		switch offset {
		case "0":
			w.Header().Set("X-Plex-Container-Start", "0")
			w.Header().Set("X-Plex-Container-Total-Size", "10")
			_, _ = w.Write([]byte(`<MediaContainer><Video ratingKey="1"/><Video ratingKey="2"/><Video ratingKey="3"/><Video ratingKey="4"/><Video ratingKey="5"/><Video ratingKey="6"/><Video ratingKey="7"/><Video ratingKey="8"/></MediaContainer>`))
		case "8":
			w.Header().Set("X-Plex-Container-Start", "8")
			w.Header().Set("X-Plex-Container-Total-Size", "10")
			_, _ = w.Write([]byte(`<MediaContainer><Video ratingKey="9"/><Video ratingKey="10"/></MediaContainer>`))
		default:
			t.Errorf("unexpected offset %q", offset)
		}
	}))
	defer server.Close()
	items, err := NewClient(server.URL, "test-token", 5*time.Second).ListPlaylistItems(context.Background(), "500")
	if err != nil {
		t.Fatalf("ListPlaylistItems failed: %v", err)
	}
	if len(items) != 10 || items[8].EpisodeID != "9" || items[9].EpisodeID != "10" {
		t.Fatalf("items = %#v", items)
	}
}

func TestUpsertPlaylistDetectsExactProjectionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<MediaContainer machineIdentifier="server-id"/>`))
		case "/playlists":
			_, _ = w.Write([]byte(`<MediaContainer><Playlist ratingKey="500" title="Rotation"/></MediaContainer>`))
		case "/playlists/500/items":
			_, _ = w.Write([]byte(`<MediaContainer size="1"><Video ratingKey="100"/></MediaContainer>`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	_, err := NewClient(server.URL, "test-token", 5*time.Second).UpsertPlaylist(context.Background(), nil, "Rotation", []string{"100", "101"})
	var mismatch *PublicationMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %v, want PublicationMismatchError", err)
	}
	if len(mismatch.Missing) != 1 || mismatch.Missing[0] != "101" {
		t.Fatalf("mismatch = %#v", mismatch)
	}
}

func TestListEpisodesWithDates(t *testing.T) {
	server, client := testServer(t, "episodes_with_dates.xml")
	defer server.Close()

	episodes, err := client.ListEpisodes(context.Background(), "20")
	if err != nil {
		t.Fatalf("ListEpisodes failed: %v", err)
	}

	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(episodes))
	}

	if episodes[0].AirDate != "2015-12-14" {
		t.Errorf("expected air date '2015-12-14', got %q", episodes[0].AirDate)
	}
	if episodes[1].AirDate != "" {
		t.Errorf("expected empty air date for episode without originallyAvailableAt, got %q", episodes[1].AirDate)
	}
}

func episodePageXML(start, end int) string {
	var b strings.Builder
	b.WriteString("<MediaContainer>")
	for i := start; i < end; i++ {
		b.WriteString(`<Video ratingKey="`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`" title="Episode `)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`"/>`)
	}
	b.WriteString("</MediaContainer>")
	return b.String()
}

func TestListEpisodesConsumesLargeUnpaginatedResponseOnce(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(episodePageXML(1, 174)))
	}))
	defer server.Close()

	episodes, err := NewClient(server.URL, "test-token", time.Second).ListEpisodes(context.Background(), "10")
	if err != nil {
		t.Fatalf("ListEpisodes failed: %v", err)
	}
	if len(episodes) != 173 || requests != 1 {
		t.Fatalf("got %d episodes in %d requests, want 173 in 1", len(episodes), requests)
	}
}

func TestListEpisodesUsesHeaderPagination(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		start, err := strconv.Atoi(r.Header.Get("X-Plex-Container-Start"))
		if err != nil || start != (requests-1)*100 {
			t.Errorf("request %d start = %q", requests, r.Header.Get("X-Plex-Container-Start"))
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("X-Plex-Container-Start", strconv.Itoa(start))
		w.Header().Set("X-Plex-Container-Total-Size", "247")
		end := start + 100
		if end > 247 {
			end = 247
		}
		_, _ = w.Write([]byte(episodePageXML(start+1, end+1)))
	}))
	defer server.Close()

	episodes, err := NewClient(server.URL, "test-token", time.Second).ListEpisodes(context.Background(), "10")
	if err != nil {
		t.Fatalf("ListEpisodes failed: %v", err)
	}
	if len(episodes) != 247 || requests != 3 {
		t.Fatalf("got %d episodes in %d requests, want 247 in 3", len(episodes), requests)
	}
	for i, episode := range episodes {
		if episode.ID != strconv.Itoa(i+1) {
			t.Fatalf("episode %d has ID %q", i, episode.ID)
		}
	}
}

func TestListEpisodesRejectsRepeatedPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("X-Plex-Container-Start", "0")
		w.Header().Set("X-Plex-Container-Total-Size", "173")
		_, _ = w.Write([]byte(episodePageXML(1, 101)))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, "test-token", time.Second).ListEpisodes(context.Background(), "10"); err == nil {
		t.Fatal("expected repeated page error")
	}
}

func TestListEpisodesRejectsEmptyPageBeforeDeclaredEnd(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("X-Plex-Container-Start", r.Header.Get("X-Plex-Container-Start"))
		w.Header().Set("X-Plex-Container-Total-Size", "173")
		if requests == 1 {
			_, _ = w.Write([]byte(episodePageXML(1, 101)))
			return
		}
		_, _ = w.Write([]byte(`<MediaContainer/>`))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, "test-token", time.Second).ListEpisodes(context.Background(), "10"); err == nil {
		t.Fatal("expected incomplete pagination error")
	}
}

func TestListEpisodesCancellationStopsRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := NewClient(server.URL, "test-token", 10*time.Second).ListEpisodes(ctx, "10")
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request did not return promptly")
	}
}

func TestTestConnection(t *testing.T) {
	server, client := testServer(t, "libraries.xml")
	defer server.Close()

	err := client.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
}
