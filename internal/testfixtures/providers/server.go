// Package providers implements loopback-only test fixtures, never production clients.
package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Server struct {
	mu      sync.Mutex
	Mode    string
	lists   map[string][]string
	markers map[string]string
}

func New() *Server { return &Server{lists: map[string][]string{}, markers: map[string]string{}} }
func (s *Server) Scenario(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Mode string `json:"mode"`
	}
	json.NewDecoder(r.Body).Decode(&b)
	s.mu.Lock()
	s.Mode = b.Mode
	s.mu.Unlock()
	w.WriteHeader(204)
}
func track(id string) map[string]any {
	title := "First Light"
	if strings.Contains(id, "2") {
		title = "Second Light"
	}
	return map[string]any{"id": id, "name": title, "type": "track", "artists": []any{map[string]string{"name": "Test Artist"}}, "duration_ms": 180000}
}
func video(id string) map[string]any {
	title := "First Light"
	if strings.Contains(id, "2") {
		title = "Second Light"
	}
	return map[string]any{"id": id, "snippet": map[string]string{"title": title, "channelTitle": "Test Artist - Topic", "categoryId": "10"}, "contentDetails": map[string]string{"duration": "PT3M"}, "status": map[string]string{"uploadStatus": "processed"}}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	send := func(v any) { json.NewEncoder(w).Encode(v) }
	p := r.URL.Path
	if strings.HasSuffix(p, "/auth") {
		q := r.URL.Query()
		http.Redirect(w, r, q.Get("redirect_uri")+"?state="+q.Get("state")+"&code=test", 302)
		return
	}
	if strings.HasSuffix(p, "/token") {
		send(map[string]any{"access_token": "fixture", "refresh_token": "fixture-refresh", "token_type": "Bearer", "expires_in": 3600})
		return
	}
	yt := strings.HasPrefix(p, "/youtube/")
	path := strings.TrimPrefix(strings.TrimPrefix(p, "/youtube/v3"), "/v1")
	meta := func(id string) map[string]any {
		if yt {
			return map[string]any{"id": id, "snippet": map[string]string{"title": "Owned favorites", "channelId": "owner", "description": s.markers[id]}}
		}
		return map[string]any{"id": id, "name": "Owned favorites", "snapshot_id": "v1", "owner": map[string]string{"id": "owner"}, "description": s.markers[id]}
	}
	if path == "/me" {
		send(map[string]string{"id": "owner", "display_name": "Test Listener"})
		return
	}
	if path == "/channels" {
		send(map[string]any{"items": []any{map[string]any{"id": "owner", "snippet": map[string]string{"title": "Test Listener"}}}})
		return
	}
	if path == "/search" {
		if s.Mode == "quota" {
			s.Mode = "normal"
			w.WriteHeader(403)
			send(map[string]any{"error": map[string]any{"errors": []any{map[string]string{"reason": "quotaExceeded"}}}})
			return
		}
		second := strings.Contains(strings.ToLower(r.URL.Query().Get("q")), "second")
		ids := []string{"song1"}
		if second {
			ids = []string{"song2", "song2b"}
		}
		if yt {
			items := []any{}
			for _, id := range ids {
				items = append(items, map[string]any{"id": map[string]string{"videoId": id}})
			}
			send(map[string]any{"items": items})
		} else {
			items := []any{}
			for _, id := range ids {
				items = append(items, track(id))
			}
			send(map[string]any{"tracks": map[string]any{"items": items}})
		}
		return
	}
	if path == "/videos" {
		items := []any{}
		for _, id := range strings.Split(r.URL.Query().Get("id"), ",") {
			items = append(items, video(id))
		}
		send(map[string]any{"items": items})
		return
	}
	create := r.Method == "POST" && (path == "/me/playlists" || path == "/playlists")
	if create {
		var b struct {
			Description string `json:"description"`
			Snippet     struct {
				Description string `json:"description"`
			} `json:"snippet"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		id := fmt.Sprintf("created%d", len(s.lists)+1)
		s.lists[id] = []string{}
		s.markers[id] = b.Description
		if yt {
			s.markers[id] = b.Snippet.Description
		}
		if s.Mode == "timeout_create" {
			s.Mode = "normal"
			h := w.(http.Hijacker)
			c, _, _ := h.Hijack()
			c.Close()
			return
		}
		send(meta(id))
		return
	}
	id := r.URL.Query().Get("id")
	if strings.HasPrefix(path, "/playlists/") {
		id = strings.Split(strings.TrimPrefix(path, "/playlists/"), "/")[0]
	}
	if path == "/playlistItems" {
		id = r.URL.Query().Get("playlistId")
	}
	if r.Method == "POST" && (path == "/playlistItems" || strings.HasSuffix(path, "/items")) {
		var b struct {
			URIs    []string `json:"uris"`
			Snippet struct {
				PlaylistID string `json:"playlistId"`
				Resource   struct {
					VideoID string `json:"videoId"`
				} `json:"resourceId"`
			} `json:"snippet"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if yt {
			id = b.Snippet.PlaylistID
			s.lists[id] = append(s.lists[id], b.Snippet.Resource.VideoID)
		} else {
			for _, uri := range b.URIs {
				s.lists[id] = append(s.lists[id], strings.TrimPrefix(uri, "spotify:track:"))
			}
		}
		if s.Mode == "timeout_append" {
			s.Mode = "normal"
			c, _, _ := w.(http.Hijacker).Hijack()
			c.Close()
			return
		}
		send(map[string]string{"id": "item"})
		return
	}
	if path == "/me/playlists" || path == "/playlists" && r.URL.Query().Get("mine") == "true" {
		items := []any{}
		for id := range s.lists {
			items = append(items, meta(id))
		}
		send(map[string]any{"items": items})
		return
	}
	if path == "/playlistItems" || strings.HasSuffix(path, "/items") {
		ids, exists := s.lists[id]
		if !exists {
			ids = []string{"song1", "song2", "song1"}
		}
		items := []any{}
		for pos, song := range ids {
			if yt {
				items = append(items, map[string]any{"snippet": map[string]any{"position": pos, "title": "Song", "resourceId": map[string]string{"videoId": song}}})
			} else {
				items = append(items, map[string]any{"item": track(song)})
			}
		}
		send(map[string]any{"items": items})
		return
	}
	if id != "" {
		if yt {
			send(map[string]any{"items": []any{meta(id)}})
		} else {
			send(meta(id))
		}
		return
	}
	http.NotFound(w, r)
}
