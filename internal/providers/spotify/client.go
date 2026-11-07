package spotify

import (
	"context"
	"net/http"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/providers"
	"regexp"
)

const defaultURL = "https://api.spotify.com/v1"

var validID = regexp.MustCompile(`^[A-Za-z0-9]+$`)

type Client struct{ api *providers.API }

var _ domain.Provider = (*Client)(nil)

func New(client *http.Client, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultURL
	}
	return &Client{api: providers.NewAPI(client, baseURL)}
}
func (c *Client) Identity(ctx context.Context, token domain.Token) (domain.Identity, error) {
	var raw struct {
		ID   string `json:"id"`
		Name string `json:"display_name"`
	}
	if err := c.api.Do(ctx, token, "GET", "/me", nil, &raw); err != nil {
		return domain.Identity{}, err
	}
	if raw.ID == "" {
		return domain.Identity{}, providers.Error("invalid_provider_response")
	}
	return domain.Identity{ID: raw.ID, Name: raw.Name}, nil
}

type track struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Local   bool   `json:"is_local"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Duration    int   `json:"duration_ms"`
	Explicit    *bool `json:"explicit"`
	Playable    *bool `json:"is_playable"`
	ExternalIDs struct {
		ISRC string `json:"isrc"`
	} `json:"external_ids"`
}

func (t *track) song() domain.Song {
	if t == nil {
		return domain.Song{Unavailable: "unavailable", Artists: []string{}}
	}
	artists := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		artists = append(artists, a.Name)
	}
	s := domain.Song{ID: t.ID, Title: t.Name, Artists: artists, DurationMS: t.Duration, Explicit: t.Explicit, ISRC: t.ExternalIDs.ISRC}
	if validID.MatchString(t.ID) {
		s.URL = "https://open.spotify.com/track/" + t.ID
	}
	switch {
	case t.Local:
		s.Unavailable = "local_only"
	case t.Type != "track":
		s.Unavailable = "unsupported_item"
	case t.ID == "" || t.Playable != nil && !*t.Playable:
		s.Unavailable = "unavailable"
	}
	return s
}
