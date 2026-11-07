package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"playlistconverter/internal/domain"
	"testing"
)

const trackJSON = `{"id":"track1","type":"track","name":"Song","artists":[{"name":"Artist"}],"duration_ms":180000,"explicit":false,"external_ids":{"isrc":"ISRC1"}}`

func TestClient(t *testing.T) {
	var writes []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			fmt.Fprint(w, `{"id":"owner","display_name":"Listener"}`)
		case "/playlists/source":
			fmt.Fprint(w, `{"id":"source","name":"Favorites","owner":{"id":"owner"},"snapshot_id":"v1"}`)
		case "/playlists/source/items":
			if r.URL.Query().Get("offset") == "0" {
				fmt.Fprintf(w, `{"items":[{"item":%s},{"item":null}],"next":"https://evil.example/steal"}`, trackJSON)
			} else {
				fmt.Fprintf(w, `{"items":[{"track":%s}],"next":null}`, trackJSON)
			}
		case "/search":
			if r.URL.Query().Get("type") != "track" {
				t.Error("wrong search")
			}
			fmt.Fprintf(w, `{"tracks":{"items":[%s]}}`, trackJSON)
		case "/me/playlists":
			if r.Method == "POST" {
				var body struct {
					Name        string
					Public      bool
					Description string
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.Public || body.Name != "New" || body.Description != "marker" {
					t.Errorf("bad create %#v", body)
				}
				fmt.Fprint(w, `{"id":"new","name":"New","owner":{"id":"owner"}}`)
			} else {
				fmt.Fprint(w, `{"items":[{"id":"new","name":"New","description":"marker","owner":{"id":"owner"}}],"next":null}`)
			}
		case "/playlists/new/items":
			var b struct {
				URIs []string `json:"uris"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			writes = append(writes, b.URIs...)
			fmt.Fprint(w, `{"snapshot_id":"v2"}`)
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
	if err != nil || len(p.Songs) != 3 || p.Songs[0].ISRC != "ISRC1" || p.Songs[1].Unavailable == "" || p.Songs[2].ID != "track1" {
		t.Fatalf("playlist %+v %v", p, err)
	}
	candidates, err := c.Search(ctx, token, p.Songs[0])
	if err != nil || len(candidates) != 1 {
		t.Fatalf("search %v %v", candidates, err)
	}
	made, err := c.CreatePlaylist(ctx, token, "New", "marker")
	if err != nil || made.ID != "new" {
		t.Fatalf("create %+v %v", made, err)
	}
	if err = c.Append(ctx, token, "new", []string{"track1", "track1"}); err != nil || len(writes) != 2 || writes[0] != "spotify:track:track1" {
		t.Fatalf("append %v %v", writes, err)
	}
	found, err := c.FindCreated(ctx, token, "marker")
	if err != nil || len(found) != 1 {
		t.Fatalf("find %v %v", found, err)
	}
}
func TestMissingIdentity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) }))
	defer s.Close()
	if _, err := New(s.Client(), s.URL).Identity(context.Background(), domain.Token{AccessToken: "x"}); err == nil {
		t.Fatal("empty identity accepted")
	}
}
func TestISRCFallback(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("q") == "isrc:unknown" {
			fmt.Fprint(w, `{"tracks":{"items":[]}}`)
		} else {
			fmt.Fprintf(w, `{"tracks":{"items":[%s]}}`, trackJSON)
		}
	}))
	defer s.Close()
	songs, err := New(s.Client(), s.URL).Search(context.Background(), domain.Token{AccessToken: "x"}, domain.Song{Title: "Song", Artists: []string{"Artist"}, ISRC: "unknown"})
	if err != nil || len(songs) != 1 || calls != 2 {
		t.Fatalf("fallback failed: %v %v calls=%d", songs, err, calls)
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
func TestSnapshotChangesAndMalformedCreation(t *testing.T) {
	reads := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/playlists/source":
			reads++
			fmt.Fprintf(w, `{"id":"source","owner":{"id":"owner"},"snapshot_id":"v%d"}`, reads)
		case "/playlists/source/items":
			fmt.Fprint(w, `{"items":[]}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer s.Close()
	c := New(s.Client(), s.URL)
	ctx := context.Background()
	token := domain.Token{AccessToken: "x"}
	if _, e := c.ReadPlaylist(ctx, token, "source"); e == nil {
		t.Fatal("source change ignored")
	}
	if _, e := c.CreatePlaylist(ctx, token, "New", "marker"); e == nil {
		t.Fatal("missing remote ID accepted")
	}
}
func TestUnavailableTracks(t *testing.T) {
	for _, body := range []string{`{"id":"local","type":"track","is_local":true}`, `{"id":"episode","type":"episode"}`, `{"id":"gone","type":"track","is_playable":false}`} {
		var v track
		if e := json.Unmarshal([]byte(body), &v); e != nil {
			t.Fatal(e)
		}
		if v.song().Unavailable == "" {
			t.Fatalf("unavailable accepted %s", body)
		}
	}
}
