package conversions

import (
	"net/url"
	"regexp"
	"strings"
)

var spotifyID = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)
var youtubeID = regexp.MustCompile(`^PL[A-Za-z0-9_-]{1,148}$`)

func ParseSource(raw string) (provider, id string, err error) {
	bad := &Error{Code: "invalid_playlist_url", Status: 400}
	if len(raw) > 2048 {
		return "", "", bad
	}
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return "", "", bad
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return "", "", bad
	}
	for _, v := range q {
		if len(v) > 1 {
			return "", "", bad
		}
	}
	if u.Host == "open.spotify.com" && strings.HasPrefix(u.Path, "/playlist/") {
		id = strings.TrimPrefix(u.Path, "/playlist/")
		if spotifyID.MatchString(id) {
			return "spotify", id, nil
		}
	}
	if (u.Host == "youtube.com" || u.Host == "www.youtube.com") && (u.Path == "/playlist" || u.Path == "/watch") {
		id = q.Get("list")
		if youtubeID.MatchString(id) {
			return "youtube", id, nil
		}
	}
	return "", "", bad
}
