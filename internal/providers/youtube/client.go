package youtube

import (
	"context"
	"net/http"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/providers"
	"regexp"
)

const defaultURL = "https://www.googleapis.com/youtube/v3"

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := c.api.Do(ctx, token, "GET", "/channels?part=snippet&mine=true&maxResults=50", nil, &raw); err != nil {
		return domain.Identity{}, err
	}
	if len(raw.Items) != 1 || raw.Items[0].ID == "" {
		return domain.Identity{}, providers.Error("channel_selection_required")
	}
	return domain.Identity{ID: raw.Items[0].ID, Name: raw.Items[0].Snippet.Title}, nil
}

type snippet struct {
	Title       string `json:"title"`
	ChannelID   string `json:"channelId"`
	Description string `json:"description"`
}
type playlist struct {
	ID      string  `json:"id"`
	Snippet snippet `json:"snippet"`
}

func (p playlist) domain() domain.Playlist {
	return domain.Playlist{ID: p.ID, Name: p.Snippet.Title, OwnerID: p.Snippet.ChannelID, URL: "https://www.youtube.com/playlist?list=" + p.ID, Songs: []domain.Song{}}
}
