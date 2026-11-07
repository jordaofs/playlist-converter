package youtube

import (
	"context"
	"html"
	"net/url"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/providers"
	"regexp"
	"strconv"
	"strings"
)

var durationPattern = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?$`)

func durationMS(raw string) int {
	m := durationPattern.FindStringSubmatch(raw)
	if m == nil {
		return 0
	}
	var total float64
	for i, factor := range []float64{3600, 60, 1} {
		n, _ := strconv.ParseFloat(m[i+1], 64)
		total += n * factor
	}
	if total > 24*3600 {
		return 0
	}
	return int(total * 1000)
}

type video struct {
	ID      string `json:"id"`
	Snippet struct {
		Title        string `json:"title"`
		ChannelTitle string `json:"channelTitle"`
		Category     string `json:"categoryId"`
		Live         string `json:"liveBroadcastContent"`
	} `json:"snippet"`
	Content struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	Status struct {
		Upload  string `json:"uploadStatus"`
		Privacy string `json:"privacyStatus"`
	} `json:"status"`
}

func (v video) song() domain.Song {
	title := html.UnescapeString(v.Snippet.Title)
	channel := html.UnescapeString(v.Snippet.ChannelTitle)
	artist := ""
	if strings.HasSuffix(channel, " - Topic") {
		artist = strings.TrimSuffix(channel, " - Topic")
	}
	if parts := strings.SplitN(title, " - ", 2); len(parts) == 2 && (strings.EqualFold(parts[0], artist) || strings.EqualFold(parts[0], channel)) {
		artist = parts[0]
		title = parts[1]
	}
	artists := []string{}
	if artist != "" {
		artists = append(artists, artist)
	}
	s := domain.Song{ID: v.ID, Title: title, Artists: artists, DurationMS: durationMS(v.Content.Duration), URL: "https://www.youtube.com/watch?v=" + v.ID}
	switch {
	case v.Snippet.Category != "10":
		s.Unavailable = "nonmusic_or_unknown_category"
	case v.Snippet.Live == "live" || v.Snippet.Live == "upcoming":
		s.Unavailable = "live_stream"
	case v.Status.Upload != "" && v.Status.Upload != "processed":
		s.Unavailable = "unavailable"
	}
	return s
}
func (c *Client) hydrate(ctx context.Context, token domain.Token, ids []string) (map[string]domain.Song, error) {
	songs := map[string]domain.Song{}
	unique := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			unique = append(unique, id)
			seen[id] = true
		}
	}
	for start := 0; start < len(unique); start += 50 {
		end := min(start+50, len(unique))
		q := url.Values{"part": {"snippet,contentDetails,status"}, "id": {strings.Join(unique[start:end], ",")}}
		var raw struct {
			Items []video `json:"items"`
		}
		if err := c.api.Do(ctx, token, "GET", "/videos?"+q.Encode(), nil, &raw); err != nil {
			return nil, err
		}
		for _, v := range raw.Items {
			if seen[v.ID] && validID.MatchString(v.ID) {
				songs[v.ID] = v.song()
			}
		}
	}
	return songs, nil
}
func (c *Client) Search(ctx context.Context, token domain.Token, source domain.Song) ([]domain.Song, error) {
	if strings.TrimSpace(source.Title) == "" {
		return nil, providers.Error("invalid_search")
	}
	q := url.Values{"part": {"snippet"}, "type": {"video"}, "videoCategoryId": {"10"}, "maxResults": {"10"}, "q": {strings.Join(source.Artists, " ") + " " + source.Title}}
	var raw struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	if err := c.api.Do(ctx, token, "GET", "/search?"+q.Encode(), nil, &raw); err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, item := range raw.Items {
		id := item.ID.VideoID
		if validID.MatchString(id) && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	hydrated, err := c.hydrate(ctx, token, ids)
	if err != nil {
		return nil, err
	}
	songs := []domain.Song{}
	for _, id := range ids {
		if s, ok := hydrated[id]; ok && s.Unavailable == "" {
			songs = append(songs, s)
		}
	}
	return songs, nil
}
