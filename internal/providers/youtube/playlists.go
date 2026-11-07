package youtube

import (
	"context"
	"errors"
	"net/url"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/providers"
)

func (c *Client) ReadPlaylist(ctx context.Context, token domain.Token, id string) (domain.Playlist, error) {
	if !validID.MatchString(id) {
		return domain.Playlist{}, providers.Error("invalid_playlist_id")
	}
	var meta struct {
		Items []playlist `json:"items"`
	}
	if err := c.api.Do(ctx, token, "GET", "/playlists?part=snippet&id="+url.QueryEscape(id), nil, &meta); err != nil {
		return domain.Playlist{}, err
	}
	if len(meta.Items) != 1 || meta.Items[0].ID != id || meta.Items[0].Snippet.ChannelID == "" {
		return domain.Playlist{}, providers.Error("not_found")
	}
	p := meta.Items[0].domain()
	pageToken := ""
	seen := map[string]bool{}
	for pageNo := 0; pageNo < 20; pageNo++ {
		var page struct {
			Items []struct {
				Snippet struct {
					Title      string `json:"title"`
					Position   int    `json:"position"`
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
			Next string `json:"nextPageToken"`
		}
		query := url.Values{"part": {"snippet"}, "playlistId": {id}, "maxResults": {"50"}, "pageToken": {pageToken}}
		if err := c.api.Do(ctx, token, "GET", "/playlistItems?"+query.Encode(), nil, &page); err != nil {
			return domain.Playlist{}, err
		}
		ids := []string{}
		for _, item := range page.Items {
			if validID.MatchString(item.Snippet.ResourceID.VideoID) {
				ids = append(ids, item.Snippet.ResourceID.VideoID)
			}
		}
		videos, err := c.hydrate(ctx, token, ids)
		if err != nil {
			return domain.Playlist{}, err
		}
		for _, item := range page.Items {
			if item.Snippet.Position != len(p.Songs) {
				return domain.Playlist{}, providers.Error("source_changed")
			}
			id := item.Snippet.ResourceID.VideoID
			s, ok := videos[id]
			if !ok {
				s = domain.Song{ID: id, Title: item.Snippet.Title, Artists: []string{}, Unavailable: "unavailable"}
			}
			p.Songs = append(p.Songs, s)
		}
		if len(p.Songs) > providers.MaxItems {
			return domain.Playlist{}, providers.Error("playlist_too_large")
		}
		if page.Next == "" {
			return p, nil
		}
		if seen[page.Next] || len(page.Items) == 0 {
			return domain.Playlist{}, providers.Error("invalid_provider_response")
		}
		seen[page.Next] = true
		pageToken = page.Next
	}
	return domain.Playlist{}, providers.Error("playlist_too_large")
}
func (c *Client) CreatePlaylist(ctx context.Context, token domain.Token, name, marker string) (domain.Playlist, error) {
	if name == "" || len([]rune(name)) > 100 || marker == "" || len(marker) > 300 {
		return domain.Playlist{}, providers.Error("invalid_playlist_name")
	}
	var p playlist
	body := map[string]any{"snippet": map[string]string{"title": name, "description": marker}, "status": map[string]string{"privacyStatus": "private"}}
	if err := c.api.Do(ctx, token, "POST", "/playlists?part=snippet,status", body, &p); err != nil {
		return domain.Playlist{}, err
	}
	if !validID.MatchString(p.ID) {
		return domain.Playlist{}, &domain.ProviderError{Code: "invalid_provider_response", UnknownOutcome: true}
	}
	return p.domain(), nil
}
func (c *Client) Append(ctx context.Context, token domain.Token, id string, ids []string) error {
	if !validID.MatchString(id) || len(ids) == 0 || len(ids) > 100 {
		return providers.Error("invalid_provider_request")
	}
	for _, videoID := range ids {
		if !validID.MatchString(videoID) {
			return providers.Error("invalid_track_id")
		}
	}
	for i, videoID := range ids {
		body := map[string]any{"snippet": map[string]any{"playlistId": id, "resourceId": map[string]string{"kind": "youtube#video", "videoId": videoID}}}
		if err := c.api.Do(ctx, token, "POST", "/playlistItems?part=snippet", body, nil); err != nil {
			var pe *domain.ProviderError
			if i > 0 && errors.As(err, &pe) {
				return &domain.ProviderError{Code: pe.Code, RetryAfter: pe.RetryAfter, UnknownOutcome: true}
			}
			return err
		}
	}
	return nil
}
func (c *Client) FindCreated(ctx context.Context, token domain.Token, marker string) ([]domain.Playlist, error) {
	if marker == "" {
		return nil, providers.Error("invalid_provider_request")
	}
	owner, err := c.Identity(ctx, token)
	if err != nil {
		return nil, err
	}
	found := []domain.Playlist{}
	pageToken := ""
	seen := map[string]bool{}
	for pageNo := 0; pageNo < 20; pageNo++ {
		var page struct {
			Items []playlist `json:"items"`
			Next  string     `json:"nextPageToken"`
		}
		q := url.Values{"part": {"snippet"}, "mine": {"true"}, "maxResults": {"50"}, "pageToken": {pageToken}}
		if err := c.api.Do(ctx, token, "GET", "/playlists?"+q.Encode(), nil, &page); err != nil {
			return nil, err
		}
		for _, p := range page.Items {
			if p.Snippet.Description == marker && p.Snippet.ChannelID == owner.ID && validID.MatchString(p.ID) {
				found = append(found, p.domain())
			}
		}
		if page.Next == "" {
			return found, nil
		}
		if seen[page.Next] || len(page.Items) == 0 {
			return nil, providers.Error("invalid_provider_response")
		}
		seen[page.Next] = true
		pageToken = page.Next
	}
	return nil, providers.Error("playlist_scan_limit")
}
