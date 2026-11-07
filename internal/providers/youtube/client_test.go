package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"playlistconverter/internal/domain"
	"testing"
)

func TestClient(t *testing.T) {
	var writes []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels":
			fmt.Fprint(w, `{"items":[{"id":"owner","snippet":{"title":"Listener"}}]}`)
		case "/playlists":
			if r.Method == "POST" {
				var raw struct {
					Status struct {
						Privacy string `json:"privacyStatus"`
					}
					Snippet struct{ Title, Description string }
				}
				_ = json.NewDecoder(r.Body).Decode(&raw)
				if raw.Status.Privacy != "private" || raw.Snippet.Description != "marker" {
					t.Errorf("bad create %+v", raw)
				}
				fmt.Fprint(w, `{"id":"new","snippet":{"title":"New","channelId":"owner"}}`)
			} else {
				fmt.Fprint(w, `{"items":[{"id":"source","snippet":{"title":"Favorites","channelId":"owner","description":"marker"}}]}`)
			}
		case "/playlistItems":
			if r.Method == "POST" {
				var raw struct {
					Snippet struct {
						ResourceID struct {
							VideoID string `json:"videoId"`
						} `json:"resourceId"`
					}
				}
				_ = json.NewDecoder(r.Body).Decode(&raw)
				writes = append(writes, raw.Snippet.ResourceID.VideoID)
				fmt.Fprint(w, `{"id":"inserted"}`)
			} else if r.URL.Query().Get("pageToken") == "" {
				fmt.Fprint(w, `{"items":[{"snippet":{"title":"Song","position":0,"resourceId":{"videoId":"v1"}}},{"snippet":{"title":"Deleted video","position":1,"resourceId":{"videoId":"deleted"}}}],"nextPageToken":"next"}`)
			} else {
				fmt.Fprint(w, `{"items":[{"snippet":{"title":"Song","position":2,"resourceId":{"videoId":"v1"}}}]}`)
			}
		case "/videos":
			fmt.Fprint(w, `{"items":[{"id":"v1","snippet":{"title":"Artist - Song (Official Audio)","channelTitle":"Artist - Topic","categoryId":"10"},"contentDetails":{"duration":"PT3M1S"},"status":{"uploadStatus":"processed"}}]}`)
		case "/search":
			fmt.Fprint(w, `{"items":[{"id":{"videoId":"v1"}},{"id":{"videoId":"v1"}}]}`)
		default:
			t.Errorf("unexpected %s", r.URL.String())
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	c := New(s.Client(), s.URL)
	ctx := context.Background()
	token := domain.Token{AccessToken: "token"}
	id, err := c.Identity(ctx, token)
	if err != nil || id.ID != "owner" {
		t.Fatalf("identity %+v %v", id, err)
	}
	p, err := c.ReadPlaylist(ctx, token, "source")
	if err != nil || len(p.Songs) != 3 || p.Songs[0].DurationMS != 181000 || p.Songs[1].Unavailable == "" || p.Songs[2].ID != "v1" {
		t.Fatalf("playlist %+v %v", p, err)
	}
	songs, err := c.Search(ctx, token, domain.Song{Title: "Song", Artists: []string{"Artist"}})
	if err != nil || len(songs) != 1 || songs[0].Artists[0] != "Artist" {
		t.Fatalf("search %+v %v", songs, err)
	}
	made, err := c.CreatePlaylist(ctx, token, "New", "marker")
	if err != nil || made.ID != "new" {
		t.Fatalf("create %+v %v", made, err)
	}
	if err = c.Append(ctx, token, "new", []string{"v1", "v1"}); err != nil || len(writes) != 2 {
		t.Fatalf("append %v %v", writes, err)
	}
	found, err := c.FindCreated(ctx, token, "marker")
	if err != nil || len(found) != 1 {
		t.Fatalf("find %v %v", found, err)
	}
}
func TestIdentityAmbiguous(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"items":[{"id":"a"},{"id":"b"}]}`) }))
	defer s.Close()
	if _, err := New(s.Client(), s.URL).Identity(context.Background(), domain.Token{AccessToken: "x"}); err == nil {
		t.Fatal("ambiguous identity accepted")
	}
}
func TestValidationAndRemoteFailures(t *testing.T) {
	ctx := context.Background()
	token := domain.Token{AccessToken: "x"}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer s.Close()
	c := New(s.Client(), s.URL)
	cases := []func() error{
		func() error { _, e := c.ReadPlaylist(ctx, token, "../x"); return e }, func() error { _, e := c.ReadPlaylist(ctx, token, "valid"); return e },
		func() error { _, e := c.Identity(ctx, token); return e }, func() error { _, e := c.Search(ctx, token, domain.Song{}); return e }, func() error { _, e := c.Search(ctx, token, domain.Song{Title: "Song"}); return e },
		func() error { _, e := c.CreatePlaylist(ctx, token, "", "marker"); return e }, func() error { _, e := c.CreatePlaylist(ctx, token, "New", "marker"); return e },
		func() error { return c.Append(ctx, token, "valid", nil) }, func() error { return c.Append(ctx, token, "valid", []string{"../x"}) },
		func() error { _, e := c.FindCreated(ctx, token, ""); return e }, func() error { _, e := c.FindCreated(ctx, token, "marker"); return e },
	}
	for i, f := range cases {
		if e := f(); e == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if New(nil, "").api.BaseURL != defaultURL {
		t.Fatal("wrong default")
	}
}
func TestPartialAppendIsUnknown(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, `{"id":"added"}`)
	}))
	defer s.Close()
	e := New(s.Client(), s.URL).Append(context.Background(), domain.Token{AccessToken: "x"}, "playlist", []string{"v1", "v2"})
	pe, ok := e.(*domain.ProviderError)
	if !ok || !pe.UnknownOutcome || calls != 2 {
		t.Fatalf("partial append %#v", e)
	}
}
func TestVideoEvidence(t *testing.T) {
	if durationMS("PT1H2M3.5S") != 3723500 || durationMS("P1D") != 0 || durationMS("PT99H") != 0 {
		t.Fatal("invalid duration interpretation")
	}
	for _, body := range []string{`{"id":"v","snippet":{"categoryId":"20"}}`, `{"id":"v","snippet":{"categoryId":"10","liveBroadcastContent":"live"}}`, `{"id":"v","snippet":{"categoryId":"10"},"status":{"uploadStatus":"failed"}}`} {
		var v video
		json.Unmarshal([]byte(body), &v)
		if v.song().Unavailable == "" {
			t.Fatal("unsupported video accepted")
		}
	}
}
func TestMalformedPlaylistResponses(t *testing.T) {
	for _, body := range []string{`{"items":[]}`, `{"items":[{"id":"source","snippet":{"channelId":"owner"}}]}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/playlistItems" {
				fmt.Fprint(w, `{"items":[{"snippet":{"position":2,"resourceId":{"videoId":"v"}}}]}`)
			} else if r.URL.Path == "/videos" {
				fmt.Fprint(w, `{"items":[]}`)
			} else {
				fmt.Fprint(w, body)
			}
		}))
		c := New(s.Client(), s.URL)
		if _, e := c.ReadPlaylist(context.Background(), domain.Token{AccessToken: "x"}, "source"); e == nil {
			t.Fatal("missing or changed playlist accepted")
		}
		s.Close()
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) }))
	defer s.Close()
	if _, e := New(s.Client(), s.URL).CreatePlaylist(context.Background(), domain.Token{AccessToken: "x"}, "New", "marker"); e == nil {
		t.Fatal("missing remote ID accepted")
	}
}
