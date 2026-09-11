package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
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
	XMLName   xml.Name      `xml:"MediaContainer"`
	Size      int           `xml:"size,attr"`
	Offset    int           `xml:"offset,attr"`
	TotalSize int           `xml:"totalSize,attr"`
	Video     []Video       `xml:"Video"`
	Playlist  []PlaylistXML `xml:"Playlist"`
}

type pagedMediaContainer struct {
	XMLName   xml.Name      `xml:"MediaContainer"`
	Size      *int          `xml:"size,attr"`
	Offset    *int          `xml:"offset,attr"`
	TotalSize *int          `xml:"totalSize,attr"`
	Directory []Directory   `xml:"Directory"`
	Video     []Video       `xml:"Video"`
	Playlist  []PlaylistXML `xml:"Playlist"`
}

// PublicationMismatchError means Plex accepted a write but its committed
// playlist projection does not match the requested projection.
type PublicationMismatchError struct {
	Expected        []string `json:"expected"`
	Actual          []string `json:"actual"`
	Missing         []string `json:"missing"`
	Unexpected      []string `json:"unexpected"`
	OrderMismatches []string `json:"order_mismatches"`
}

func (e *PublicationMismatchError) Error() string {
	return fmt.Sprintf("plex playlist publication mismatch: expected=%d actual=%d missing=%v unexpected=%v order_mismatches=%v", len(e.Expected), len(e.Actual), e.Missing, e.Unexpected, e.OrderMismatches)
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
	return c.doRequestWithHeaders(ctx, path, queryParams, nil)
}

func (c *Client) doRequestWithHeaders(ctx context.Context, path string, queryParams map[string]string, headers map[string]string) (*http.Response, error) {
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
	for key, value := range headers {
		req.Header.Set(key, value)
	}

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
	const pageSize = 100
	allDirectories := make([]Directory, 0)
	seenIDs := make(map[string]struct{})
	for page := 0; page < maxPlexPages; page++ {
		offset := len(allDirectories)
		container, paginated, err := c.getPagedContainer(ctx, fmt.Sprintf("/library/sections/%s/all", libraryID), offset, pageSize)
		if err != nil {
			return nil, fmt.Errorf("list series: %w", err)
		}
		for _, directory := range container.Directory {
			if directory.RatingKey == "" {
				return nil, fmt.Errorf("list series: series has no rating key")
			}
			if _, exists := seenIDs[directory.RatingKey]; exists {
				return nil, fmt.Errorf("list series: duplicate series rating key %q", directory.RatingKey)
			}
			seenIDs[directory.RatingKey] = struct{}{}
			allDirectories = append(allDirectories, directory)
		}
		if done, err := paginationDone(container, paginated, offset, len(container.Directory), len(allDirectories)); err != nil {
			return nil, fmt.Errorf("list series: %w", err)
		} else if done {
			break
		}
		if page == maxPlexPages-1 {
			return nil, fmt.Errorf("list series exceeded pagination limit")
		}
	}

	series := make([]media.SeriesMetadata, 0, len(allDirectories))
	for _, d := range allDirectories {
		if d.Type != "show" {
			continue
		}
		series = append(series, media.SeriesMetadata{
			ID:          d.RatingKey,
			GUID:        d.GUID,
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
	const pageSize = 100
	allVideos := make([]Video, 0)
	seenIDs := make(map[string]struct{})
	for page := 0; page < maxPlexPages; page++ {
		offset := len(allVideos)
		container, paginated, err := c.getPagedContainer(ctx, fmt.Sprintf("/library/metadata/%s/allLeaves", seriesID), offset, pageSize)
		if err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		}
		for _, video := range container.Video {
			if video.RatingKey == "" {
				return nil, fmt.Errorf("list episodes: episode has no rating key")
			}
			if _, exists := seenIDs[video.RatingKey]; exists {
				return nil, fmt.Errorf("list episodes: duplicate episode rating key %q", video.RatingKey)
			}
			seenIDs[video.RatingKey] = struct{}{}
			allVideos = append(allVideos, video)
		}
		if done, err := paginationDone(container, paginated, offset, len(container.Video), len(allVideos)); err != nil {
			return nil, fmt.Errorf("list episodes: %w", err)
		} else if done {
			break
		}
		if page == maxPlexPages-1 {
			return nil, fmt.Errorf("list episodes exceeded pagination limit")
		}
	}
	episodes := make([]media.EpisodeMetadata, 0, len(allVideos))
	for _, v := range allVideos {
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
				EpisodeID:    v.RatingKey,
				ViewCount:    v.ViewCount,
				ViewOffset:   v.ViewOffset,
				LastViewedAt: v.LastViewedAt,
			}
			if v.ViewOffset > 0 && v.ViewCount == 0 {
				// Plex exposes a non-zero offset while an unwatched item has playback progress.
				progress.Watching = true
			}
			allProgress = append(allProgress, progress)
		}
	}

	return allProgress, nil
}

func (c *Client) ListPlaylistItems(ctx context.Context, playlistID string) ([]media.PlaylistItem, error) {
	const pageSize = 100
	items := make([]media.PlaylistItem, 0)
	for page := 0; page < maxPlexPages; page++ {
		offset := len(items)
		container, paginated, err := c.getPagedContainer(ctx, fmt.Sprintf("/playlists/%s/items", playlistID), offset, pageSize)
		if err != nil {
			return nil, fmt.Errorf("list playlist items page offset %d: %w", offset, err)
		}
		for _, v := range container.Video {
			if v.RatingKey == "" {
				return nil, fmt.Errorf("playlist item has no rating key")
			}
			items = append(items, media.PlaylistItem{EpisodeID: v.RatingKey, SeriesTitle: v.GrandparentTitle, EpisodeTitle: v.Title, SeasonNumber: v.ParentIndex, EpisodeNumber: v.Index})
		}

		if done, err := paginationDone(container, paginated, offset, len(container.Video), len(items)); err != nil {
			return nil, fmt.Errorf("list playlist items page offset %d: %w", offset, err)
		} else if done {
			return items, nil
		}
	}
	return nil, fmt.Errorf("list playlist items exceeded pagination limit")
}

const maxPlexPages = 1000

func (c *Client) getPagedContainer(ctx context.Context, path string, offset, pageSize int) (pagedMediaContainer, bool, error) {
	resp, err := c.doRequestWithHeaders(ctx, path, nil, map[string]string{
		"X-Plex-Container-Start": strconv.Itoa(offset),
		"X-Plex-Container-Size":  strconv.Itoa(pageSize),
	})
	if err != nil {
		return pagedMediaContainer{}, false, err
	}
	defer resp.Body.Close()
	var container pagedMediaContainer
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return pagedMediaContainer{}, false, fmt.Errorf("decode response: %w", err)
	}
	responsePagination := false
	for header, target := range map[string]**int{
		"X-Plex-Container-Start":      &container.Offset,
		"X-Plex-Container-Size":       &container.Size,
		"X-Plex-Container-Total-Size": &container.TotalSize,
	} {
		if targetValue := resp.Header.Get(header); targetValue != "" {
			value, parseErr := strconv.Atoi(targetValue)
			if parseErr != nil || value < 0 {
				return pagedMediaContainer{}, false, fmt.Errorf("invalid %s response header %q", header, targetValue)
			}
			if *target == nil {
				*target = &value
			}
			responsePagination = true
		}
	}
	return container, responsePagination || container.Offset != nil || container.TotalSize != nil, nil
}

func paginationDone(container pagedMediaContainer, paginated bool, requestedOffset, returned, collected int) (bool, error) {
	if !paginated {
		return true, nil
	}
	if container.Offset != nil {
		if *container.Offset != requestedOffset {
			return false, fmt.Errorf("pagination returned offset %d for requested offset %d", *container.Offset, requestedOffset)
		}
	} else if requestedOffset != 0 {
		return false, fmt.Errorf("pagination response omitted offset for requested offset %d", requestedOffset)
	}
	if container.TotalSize != nil {
		if *container.TotalSize < collected {
			return false, fmt.Errorf("pagination total size %d is less than collected %d", *container.TotalSize, collected)
		}
		if returned == 0 && collected < *container.TotalSize {
			return false, fmt.Errorf("pagination ended at %d of declared %d items", collected, *container.TotalSize)
		}
		return collected == *container.TotalSize, nil
	}
	if returned == 0 || returned < 100 {
		return true, nil
	}
	return false, nil
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
	slog.Info("plex playlist create response", "status", resp.StatusCode)

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
	slog.Info("plex playlist create metadata", "status", resp.StatusCode, "playlist_id", container.Playlist[0].RatingKey, "leaf_count", container.Playlist[0].LeafCount)
	playlist := media.Playlist{
		ID:      container.Playlist[0].RatingKey,
		Name:    container.Playlist[0].Title,
		ItemIDs: episodeIDs,
	}
	if err := c.verifyPlaylist(ctx, playlist.ID, episodeIDs); err != nil {
		return media.Playlist{}, err
	}
	return playlist, nil
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
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return media.Playlist{}, fmt.Errorf("read Plex update response: %w", readErr)
	}
	var responseContainer struct {
		Size      int `xml:"size,attr"`
		TotalSize int `xml:"totalSize,attr"`
		LeafCount int `xml:"leafCount,attr"`
	}
	if len(responseBody) > 0 {
		if err := xml.Unmarshal(responseBody, &responseContainer); err != nil {
			return media.Playlist{}, fmt.Errorf("decode Plex update response: %w", err)
		}
	}
	slog.Info("plex playlist update response", "status", resp.StatusCode, "size", responseContainer.Size, "total_size", responseContainer.TotalSize, "leaf_count", responseContainer.LeafCount)

	playlist := media.Playlist{
		ID:      playlistID,
		Name:    name,
		ItemIDs: episodeIDs,
	}
	if err := c.verifyPlaylist(ctx, playlistID, episodeIDs); err != nil {
		return media.Playlist{}, err
	}
	return playlist, nil
}

func (c *Client) verifyPlaylist(ctx context.Context, playlistID string, expected []string) error {
	items, err := c.ListPlaylistItems(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("verify playlist %s: %w", playlistID, err)
	}
	actual := make([]string, len(items))
	for i, item := range items {
		actual[i] = item.EpisodeID
	}
	if equalStrings(expected, actual) {
		return nil
	}

	expectedSet := make(map[string]bool, len(expected))
	actualSet := make(map[string]bool, len(actual))
	for _, id := range expected {
		expectedSet[id] = true
	}
	for _, id := range actual {
		actualSet[id] = true
	}
	missing, unexpected := []string{}, []string{}
	for _, id := range expected {
		if !actualSet[id] {
			missing = append(missing, id)
		}
	}
	for _, id := range actual {
		if !expectedSet[id] {
			unexpected = append(unexpected, id)
		}
	}
	order := []string{}
	for i := 0; i < len(expected) && i < len(actual); i++ {
		if expected[i] != actual[i] {
			order = append(order, fmt.Sprintf("%d:%s!=%s", i, expected[i], actual[i]))
		}
	}
	return &PublicationMismatchError{Expected: expected, Actual: actual, Missing: missing, Unexpected: unexpected, OrderMismatches: order}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
