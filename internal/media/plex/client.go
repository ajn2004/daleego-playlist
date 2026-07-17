package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andrew/rotator/internal/media"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type MediaContainer struct {
	XMLName  xml.Name      `xml:"MediaContainer"`
	Size     int           `xml:"size,attr"`
	Video    []Video       `xml:"Video"`
	Playlist []PlaylistXML `xml:"Playlist"`
}

type Video struct {
	XMLName               xml.Name `xml:"Video"`
	RatingKey             string   `xml:"ratingKey,attr"`
	Key                   string   `xml:"key,attr"`
	ParentRatingKey       string   `xml:"parentRatingKey,attr"`
	GrandparentRatingKey  string   `xml:"grandparentRatingKey,attr"`
	GUID                  string   `xml:"guid,attr"`
	Type                  string   `xml:"type,attr"`
	Title                 string   `xml:"title,attr"`
	GrandparentTitle      string   `xml:"grandparentTitle,attr"`
	TitleSort             string   `xml:"titleSort,attr"`
	ContentRating         string   `xml:"contentRating,attr"`
	Summary               string   `xml:"summary,attr"`
	Index                 int      `xml:"index,attr"`
	ParentIndex           int      `xml:"parentIndex,attr"`
	Year                  int      `xml:"year,attr"`
	Duration              int      `xml:"duration,attr"`
	Rating                float64  `xml:"rating,attr"`
	AudienceRating        float64  `xml:"audienceRating,attr"`
	ViewCount             int      `xml:"viewCount,attr"`
	ViewOffset            int      `xml:"viewOffset,attr"`
	OriginallyAvailableAt string   `xml:"originallyAvailableAt,attr"`
	LibrarySectionID      string   `xml:"librarySectionID,attr"`
	LibrarySectionTitle   string   `xml:"librarySectionTitle,attr"`
	Media                 []Media  `xml:"Media"`
	UserRating            float64  `xml:"userRating,attr"`
	LastViewedAt          int64    `xml:"lastViewedAt,attr"`
}

type Media struct {
	XMLName         xml.Name `xml:"Media"`
	ID              int      `xml:"id,attr"`
	Duration        int      `xml:"duration,attr"`
	Bitrate         int      `xml:"bitrate,attr"`
	AudioChannels   int      `xml:"audioChannels,attr"`
	AudioCodec      string   `xml:"audioCodec,attr"`
	VideoCodec      string   `xml:"videoCodec,attr"`
	VideoResolution string   `xml:"videoResolution,attr"`
	Container       string   `xml:"container,attr"`
	Part            []Part   `xml:"Part"`
}

type Part struct {
	XMLName  xml.Name `xml:"Part"`
	ID       int      `xml:"id,attr"`
	Key      string   `xml:"key,attr"`
	Duration int      `xml:"duration,attr"`
	File     string   `xml:"file,attr"`
	Size     int64    `xml:"size,attr"`
}

type PlaylistXML struct {
	XMLName      xml.Name `xml:"Playlist"`
	RatingKey    string   `xml:"ratingKey,attr"`
	Key          string   `xml:"key,attr"`
	Title        string   `xml:"title,attr"`
	Composite    string   `xml:"composite,attr"`
	Summary      string   `xml:"summary,attr"`
	Duration     int      `xml:"duration,attr"`
	LeafCount    int      `xml:"leafCount,attr"`
	PlaylistType string   `xml:"playlistType,attr"`
	Smart        int      `xml:"smart,attr"`
}

type Directory struct {
	XMLName             xml.Name `xml:"Directory"`
	RatingKey           string   `xml:"ratingKey,attr"`
	Key                 string   `xml:"key,attr"`
	Title               string   `xml:"title,attr"`
	Type                string   `xml:"type,attr"`
	GUID                string   `xml:"guid,attr"`
	Summary             string   `xml:"summary,attr"`
	Index               int      `xml:"index,attr"`
	Year                int      `xml:"year,attr"`
	Duration            int      `xml:"duration,attr"`
	LeafCount           int      `xml:"leafCount,attr"`
	ViewedLeafCount     int      `xml:"viewedLeafCount,attr"`
	ChildCount          int      `xml:"childCount,attr"`
	LibrarySectionID    string   `xml:"librarySectionID,attr"`
	LibrarySectionTitle string   `xml:"librarySectionTitle,attr"`
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) doRequest(ctx context.Context, path string, queryParams map[string]string) (*http.Response, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	q.Set("X-Plex-Token", c.token)
	for k, v := range queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("plex returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (c *Client) doPost(ctx context.Context, path string, body io.Reader, contentType string) (*http.Response, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("X-Plex-Token", c.token)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

func (c *Client) doPut(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("X-Plex-Token", c.token)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

func (c *Client) doDelete(ctx context.Context, path string) (*http.Response, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("X-Plex-Token", c.token)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

func (c *Client) playlistURI(ctx context.Context, episodeIDs []string) (string, error) {
	resp, err := c.doRequest(ctx, "/", nil)
	if err != nil {
		return "", fmt.Errorf("get Plex server identity: %w", err)
	}
	defer resp.Body.Close()

	var container struct {
		MachineIdentifier string `xml:"machineIdentifier,attr"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", fmt.Errorf("decode Plex server identity: %w", err)
	}
	if container.MachineIdentifier == "" {
		return "", fmt.Errorf("Plex server response has no machine identifier")
	}

	return fmt.Sprintf("server://%s/com.plexapp.plugins.library/library/metadata/%s", container.MachineIdentifier, strings.Join(episodeIDs, ",")), nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "/", nil)
	if err != nil {
		return fmt.Errorf("plex connection: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) ListLibraries(ctx context.Context) ([]media.Library, error) {
	resp, err := c.doRequest(ctx, "/library/sections", nil)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	defer resp.Body.Close()

	var container struct {
		XMLName   xml.Name    `xml:"MediaContainer"`
		Directory []Directory `xml:"Directory"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decode libraries: %w", err)
	}

	libraries := make([]media.Library, 0, len(container.Directory))
	for _, d := range container.Directory {
		id := d.RatingKey
		if id == "" {
			id = parseIDFromKey(d.Key)
		}
		libraries = append(libraries, media.Library{
			ID:    id,
			Title: d.Title,
			Type:  d.Type,
		})
	}
	return libraries, nil
}

func (c *Client) ListSeries(ctx context.Context, libraryID string) ([]media.SeriesMetadata, error) {
	resp, err := c.doRequest(ctx, fmt.Sprintf("/library/sections/%s/all", libraryID), nil)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	defer resp.Body.Close()

	var container struct {
		XMLName   xml.Name    `xml:"MediaContainer"`
		Directory []Directory `xml:"Directory"`
		Video     []Video     `xml:"Video"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decode series: %w", err)
	}

	series := make([]media.SeriesMetadata, 0, len(container.Directory))
	for _, d := range container.Directory {
		if d.Type != "show" {
			continue
		}
		series = append(series, media.SeriesMetadata{
			ID:          d.RatingKey,
			Title:       d.Title,
			Summary:     d.Summary,
			Year:        d.Year,
			LibraryID:   libraryID,
			LibraryName: "",
		})
	}
	return series, nil
}

func (c *Client) ListEpisodes(ctx context.Context, seriesID string) ([]media.EpisodeMetadata, error) {
	resp, err := c.doRequest(ctx, fmt.Sprintf("/library/metadata/%s/allLeaves", seriesID), nil)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer resp.Body.Close()

	var container struct {
		XMLName xml.Name `xml:"MediaContainer"`
		Video   []Video  `xml:"Video"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decode episodes: %w", err)
	}

	episodes := make([]media.EpisodeMetadata, 0, len(container.Video))
	for _, v := range container.Video {
		rating := v.Rating
		if rating == 0 {
			rating = v.AudienceRating
		}
		duration := v.Duration / 1000
		if len(v.Media) > 0 && v.Media[0].Duration > 0 {
			duration = v.Media[0].Duration / 1000
		}
		episodes = append(episodes, media.EpisodeMetadata{
			ID:            v.RatingKey,
			SeriesID:      seriesID,
			Title:         v.Title,
			SeasonNumber:  v.ParentIndex,
			EpisodeNumber: v.Index,
			AbsoluteOrder: 0,
			Duration:      duration,
			Rating:        rating,
			AirDate:       v.OriginallyAvailableAt,
		})
	}
	return episodes, nil
}

func (c *Client) GetEpisodeProgress(ctx context.Context, episodeIDs []string) ([]media.EpisodeProgress, error) {
	if len(episodeIDs) == 0 {
		return nil, nil
	}

	var allProgress []media.EpisodeProgress

	for _, id := range episodeIDs {
		resp, err := c.doRequest(ctx, fmt.Sprintf("/library/metadata/%s", id), nil)
		if err != nil {
			return nil, fmt.Errorf("get episode progress: %w", err)
		}

		var container struct {
			XMLName xml.Name `xml:"MediaContainer"`
			Video   []Video  `xml:"Video"`
		}
		if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode episode progress: %w", err)
		}
		resp.Body.Close()

		for _, v := range container.Video {
			progress := media.EpisodeProgress{
				EpisodeID:  v.RatingKey,
				ViewCount:  v.ViewCount,
				ViewOffset: v.ViewOffset,
			}
			if v.ViewCount > 0 {
				progress.Viewed = true
			} else if v.ViewOffset > 0 {
				// Plex exposes a non-zero offset while an unwatched item has playback progress.
				progress.Watching = true
			}
			allProgress = append(allProgress, progress)
		}
	}

	return allProgress, nil
}

func (c *Client) ListPlaylistItems(ctx context.Context, playlistID string) ([]media.PlaylistItem, error) {
	resp, err := c.doRequest(ctx, fmt.Sprintf("/playlists/%s/items", playlistID), nil)
	if err != nil {
		return nil, fmt.Errorf("list playlist items: %w", err)
	}
	defer resp.Body.Close()

	var container struct {
		XMLName xml.Name `xml:"MediaContainer"`
		Video   []Video  `xml:"Video"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decode playlist items: %w", err)
	}

	items := make([]media.PlaylistItem, 0, len(container.Video))
	for _, v := range container.Video {
		if v.RatingKey == "" {
			return nil, fmt.Errorf("playlist item has no rating key")
		}
		items = append(items, media.PlaylistItem{
			EpisodeID:     v.RatingKey,
			SeriesTitle:   v.GrandparentTitle,
			EpisodeTitle:  v.Title,
			SeasonNumber:  v.ParentIndex,
			EpisodeNumber: v.Index,
		})
	}

	return items, nil
}

func (c *Client) ClearPlaylistItems(ctx context.Context, playlistID string) error {
	items, err := c.ListPlaylistItems(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("list playlist items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	episodeIDs := make([]string, len(items))
	for i, item := range items {
		episodeIDs[i] = item.EpisodeID
	}
	playlistURI, err := c.playlistURI(ctx, episodeIDs)
	if err != nil {
		return fmt.Errorf("build playlist item URI: %w", err)
	}
	resp, err := c.doDelete(ctx, fmt.Sprintf("/playlists/%s/items?uri=%s", playlistID, url.QueryEscape(playlistURI)))
	if err != nil {
		return fmt.Errorf("clear playlist items: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Plex clear playlist returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) UpsertPlaylist(ctx context.Context, playlistID *string, name string, episodeIDs []string) (media.Playlist, error) {
	if len(episodeIDs) == 0 {
		return media.Playlist{}, fmt.Errorf("at least one episode required")
	}

	if playlistID != nil && *playlistID != "" {
		return c.updatePlaylist(ctx, *playlistID, name, episodeIDs)
	}
	return c.createPlaylist(ctx, name, episodeIDs)
}

func (c *Client) createPlaylist(ctx context.Context, name string, episodeIDs []string) (media.Playlist, error) {
	playlistURI, err := c.playlistURI(ctx, episodeIDs)
	if err != nil {
		return media.Playlist{}, err
	}

	params := url.Values{}
	params.Set("title", name)
	params.Set("type", "video")
	params.Set("smart", "0")
	params.Set("uri", playlistURI)

	u, _ := url.Parse(c.baseURL + "/playlists")
	q := u.Query()
	q.Set("X-Plex-Token", c.token)
	for k, v := range params {
		q[k] = v
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return media.Playlist{}, fmt.Errorf("create playlist request: %w", err)
	}
	req.Header.Set("Accept", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return media.Playlist{}, fmt.Errorf("create playlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return media.Playlist{}, fmt.Errorf("plex create playlist returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var container struct {
		XMLName  xml.Name      `xml:"MediaContainer"`
		Playlist []PlaylistXML `xml:"Playlist"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return media.Playlist{}, fmt.Errorf("decode playlist response: %w", err)
	}

	if len(container.Playlist) == 0 {
		return media.Playlist{}, fmt.Errorf("no playlist in response")
	}

	return media.Playlist{
		ID:      container.Playlist[0].RatingKey,
		Name:    container.Playlist[0].Title,
		ItemIDs: episodeIDs,
	}, nil
}

func (c *Client) updatePlaylist(ctx context.Context, playlistID, name string, episodeIDs []string) (media.Playlist, error) {
	// Plex's item update endpoint appends items. Clear first so this projection
	// exactly matches the local queue and watched items do not remain behind.
	if err := c.ClearPlaylistItems(ctx, playlistID); err != nil {
		return media.Playlist{}, fmt.Errorf("clear existing playlist items: %w", err)
	}

	playlistURI, err := c.playlistURI(ctx, episodeIDs)
	if err != nil {
		return media.Playlist{}, err
	}

	resp, err := c.doPut(ctx, fmt.Sprintf("/playlists/%s/items?uri=%s", playlistID, url.QueryEscape(playlistURI)), nil)
	if err != nil {
		return media.Playlist{}, fmt.Errorf("update playlist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return media.Playlist{}, fmt.Errorf("plex update playlist returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return media.Playlist{
		ID:      playlistID,
		Name:    name,
		ItemIDs: episodeIDs,
	}, nil
}

// parseIDFromKey extracts the numeric rating key from a Plex key path.
func parseIDFromKey(key string) string {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}

// strconv helpers

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
