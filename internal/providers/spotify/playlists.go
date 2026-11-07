package spotify

import (
	"context"
	"fmt"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/providers"
)

type playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Snapshot    string `json:"snapshot_id"`
	Owner       struct {
		ID string `json:"id"`
	} `json:"owner"`
}

func (p playlist) domain() domain.Playlist {
	return domain.Playlist{ID: p.ID, Name: p.Name, OwnerID: p.Owner.ID, Revision: p.Snapshot, URL: "https://open.spotify.com/playlist/" + p.ID, Songs: []domain.Song{}}
}
func (c *Client) ReadPlaylist(ctx context.Context, token domain.Token, id string) (domain.Playlist, error) {
	if !validID.MatchString(id) {
		return domain.Playlist{}, providers.Error("invalid_playlist_id")
	}
	var raw playlist
	if err := c.api.Do(ctx, token, "GET", "/playlists/"+id, nil, &raw); err != nil {
		return domain.Playlist{}, err
	}
	if raw.ID != id || raw.Owner.ID == "" {
		return domain.Playlist{}, providers.Error("invalid_provider_response")
	}
	p := raw.domain()
	for offset := 0; offset < providers.MaxItems; {
		var page struct {
			Items []struct {
				Item  *track `json:"item"`
				Track *track `json:"track"`
				Local bool   `json:"is_local"`
			} `json:"items"`
			Next *string `json:"next"`
		}
		path := fmt.Sprintf("/playlists/%s/items?limit=50&offset=%d", id, offset)
		if err := c.api.Do(ctx, token, "GET", path, nil, &page); err != nil {
			return domain.Playlist{}, err
		}
		for _, item := range page.Items {
			t := item.Item
			if t == nil {
				t = item.Track
			}
			s := t.song()
			if item.Local {
				s.Unavailable = "local_only"
			}
			p.Songs = append(p.Songs, s)
		}
		if len(p.Songs) > providers.MaxItems {
			return domain.Playlist{}, providers.Error("playlist_too_large")
		}
		if page.Next == nil || *page.Next == "" {
			var after playlist
			if err := c.api.Do(ctx, token, "GET", "/playlists/"+id, nil, &after); err != nil {
				return domain.Playlist{}, err
			}
			if raw.Snapshot != after.Snapshot {
				return domain.Playlist{}, providers.Error("source_changed")
			}
			return p, nil
		}
		if len(page.Items) == 0 {
			return domain.Playlist{}, providers.Error("invalid_provider_response")
		}
		offset += len(page.Items)
	}
	return domain.Playlist{}, providers.Error("playlist_too_large")
}
func (c *Client) CreatePlaylist(ctx context.Context, token domain.Token, name, marker string) (domain.Playlist, error) {
	if name == "" || len([]rune(name)) > 100 || marker == "" || len(marker) > 300 {
		return domain.Playlist{}, providers.Error("invalid_playlist_name")
	}
	var raw playlist
	err := c.api.Do(ctx, token, "POST", "/me/playlists", map[string]any{"name": name, "description": marker, "public": false}, &raw)
	if err != nil {
		return domain.Playlist{}, err
	}
	if !validID.MatchString(raw.ID) {
		return domain.Playlist{}, &domain.ProviderError{Code: "invalid_provider_response", UnknownOutcome: true}
	}
	return raw.domain(), nil
}
func (c *Client) Append(ctx context.Context, token domain.Token, id string, ids []string) error {
	if !validID.MatchString(id) || len(ids) == 0 || len(ids) > 100 {
		return providers.Error("invalid_provider_request")
	}
	uris := make([]string, len(ids))
	for i, trackID := range ids {
		if !validID.MatchString(trackID) {
			return providers.Error("invalid_track_id")
		}
		uris[i] = "spotify:track:" + trackID
	}
	return c.api.Do(ctx, token, "POST", "/playlists/"+id+"/items", map[string]any{"uris": uris}, nil)
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
	for offset := 0; offset < providers.MaxItems; {
		var page struct {
			Items []playlist `json:"items"`
			Next  *string    `json:"next"`
		}
		if err := c.api.Do(ctx, token, "GET", fmt.Sprintf("/me/playlists?limit=50&offset=%d", offset), nil, &page); err != nil {
			return nil, err
		}
		for _, p := range page.Items {
			if p.Description == marker && p.Owner.ID == owner.ID && validID.MatchString(p.ID) {
				found = append(found, p.domain())
			}
		}
		if page.Next == nil || *page.Next == "" {
			return found, nil
		}
		if len(page.Items) == 0 {
			return nil, providers.Error("invalid_provider_response")
		}
		offset += len(page.Items)
	}
	return nil, providers.Error("playlist_scan_limit")
}
