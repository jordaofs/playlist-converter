package spotify

import (
	"context"
	"net/url"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/providers"
	"strings"
)

func (c *Client) Search(ctx context.Context, token domain.Token, source domain.Song) ([]domain.Song, error) {
	if strings.TrimSpace(source.Title) == "" {
		return nil, providers.Error("invalid_search")
	}
	q := source.Title + " " + strings.Join(source.Artists, " ")
	if source.ISRC != "" {
		q = "isrc:" + source.ISRC
	}
	values := url.Values{"q": {q}, "type": {"track"}, "limit": {"10"}}
	var raw struct {
		Tracks struct {
			Items []track `json:"items"`
		} `json:"tracks"`
	}
	if err := c.api.Do(ctx, token, "GET", "/search?"+values.Encode(), nil, &raw); err != nil {
		return nil, err
	}
	if len(raw.Tracks.Items) == 0 && source.ISRC != "" {
		values.Set("q", source.Title+" "+strings.Join(source.Artists, " "))
		if err := c.api.Do(ctx, token, "GET", "/search?"+values.Encode(), nil, &raw); err != nil {
			return nil, err
		}
	}
	songs := make([]domain.Song, 0, len(raw.Tracks.Items))
	seen := map[string]bool{}
	for _, t := range raw.Tracks.Items {
		s := t.song()
		if s.Unavailable == "" && !seen[s.ID] {
			songs = append(songs, s)
			seen[s.ID] = true
		}
	}
	return songs, nil
}
