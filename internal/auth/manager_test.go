package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"golang.org/x/oauth2"
	"io"
	_ "modernc.org/sqlite"
	"net/http"
	"net/http/httptest"
	"net/url"
	"playlistconverter/internal/domain"
	"strings"
	"testing"
	"time"
)

func manager(t *testing.T) *Manager {
	t.Helper()
	db, e := sql.Open("sqlite", ":memory:")
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, e = db.Exec(`CREATE TABLE principals(id TEXT PRIMARY KEY,created_at INTEGER,expires_at INTEGER); CREATE TABLE sessions(id TEXT PRIMARY KEY,principal_id TEXT,csrf TEXT,expires_at INTEGER); CREATE TABLE connections(principal_id TEXT,provider TEXT,identity_id TEXT,name TEXT,token BLOB,PRIMARY KEY(principal_id,provider)); CREATE TABLE oauth_attempts(state TEXT PRIMARY KEY,principal_id TEXT,session_id TEXT,provider TEXT,verifier TEXT,expires_at INTEGER);`)
	if e != nil {
		t.Fatal(e)
	}
	return &Manager{DB: db, Key: make([]byte, 32), KeyID: "v1", Client: &http.Client{Timeout: time.Second}, OAuth: map[string]*oauth2.Config{}, Providers: map[string]domain.Provider{}}
}
func session(t *testing.T, m *Manager) (Session, *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	s, e := m.Session(context.Background(), w, httptest.NewRequest("GET", "/", nil), true)
	if e != nil {
		t.Fatal(e)
	}
	return s, w.Result().Cookies()[0]
}
func TestSessionLifecycle(t *testing.T) {
	m := manager(t)
	s, c := session(t, m)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe cookie")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(c)
	got, e := m.Session(r.Context(), httptest.NewRecorder(), r, false)
	if e != nil || got != s {
		t.Fatal("session lost", e)
	}
	m.DB.Exec("UPDATE sessions SET expires_at=0")
	if _, e = m.Session(r.Context(), httptest.NewRecorder(), r, false); e != ErrSession {
		t.Fatal("expired accepted")
	}
	if _, e = m.Session(r.Context(), httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), false); e != ErrSession {
		t.Fatal("missing accepted")
	}
}

type identityProvider struct{ domain.Provider }

func (identityProvider) Identity(context.Context, domain.Token) (domain.Identity, error) {
	return domain.Identity{ID: "account", Name: "Artist"}, nil
}
func TestOAuthBindingAndReplay(t *testing.T) {
	m := manager(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer ts.Close()
	m.OAuth["spotify"] = &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token"}, RedirectURL: "http://localhost/callback"}
	m.Providers["spotify"] = identityProvider{}
	s, _ := session(t, m)
	link, e := m.Begin(context.Background(), s, "spotify")
	if e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(link)
	state := u.Query().Get("state")
	if state == "" || u.Query().Get("code_challenge") == "" {
		t.Fatal("state/pkce missing")
	}
	other, _ := session(t, m)
	if e = m.Finish(context.Background(), httptest.NewRecorder(), other, "spotify", state, "code"); e == nil {
		t.Fatal("cross session")
	}
	w := httptest.NewRecorder()
	if e = m.Finish(context.Background(), w, s, "spotify", state, "code"); e != nil {
		t.Fatal(e)
	}
	if e = m.Finish(context.Background(), w, s, "spotify", state, "code"); e == nil {
		t.Fatal("replay")
	}
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("not rotated")
	}
	connections, e := m.Connections(context.Background(), s.PrincipalID)
	if e != nil || len(connections) != 1 || connections[0].ID != "account" {
		t.Fatal(connections, e)
	}
	var blob []byte
	m.DB.QueryRow("SELECT token FROM connections").Scan(&blob)
	if strings.Contains(string(blob), "refresh") {
		t.Fatal("plaintext token")
	}
	token, _, e := m.Token(context.Background(), s.PrincipalID, "spotify")
	if e != nil || token.AccessToken != "access" {
		t.Fatal(e)
	}
}
func TestTokensAndRefresh(t *testing.T) {
	m := manager(t)
	s, _ := session(t, m)
	ctx := context.Background()
	if _, _, e := m.Token(ctx, s.PrincipalID, "spotify"); e != ErrConnection {
		t.Fatal(e)
	}
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"access_token": "new", "refresh_token": "rotated", "token_type": "Bearer", "expires_in": 3600})
	}))
	defer ts.Close()
	m.OAuth["spotify"] = &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{TokenURL: ts.URL}}
	e := m.Save(ctx, s.PrincipalID, "spotify", domain.Identity{ID: "account"}, domain.Token{AccessToken: "old", RefreshToken: "refresh", Expiry: time.Now().Add(-time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		v, _, e := m.Token(ctx, s.PrincipalID, "spotify")
		if e != nil || v.RefreshToken != "rotated" {
			t.Fatal(v, e)
		}
	}
	if calls != 1 {
		t.Fatal("repeated refresh")
	}
	m.Key = []byte("bad")
	if _, _, e = m.Token(ctx, s.PrincipalID, "spotify"); e != ErrConnection {
		t.Fatal("bad key")
	}
}
func TestOAuthFailurePaths(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	s, _ := session(t, m)
	if _, e := m.Begin(ctx, s, "invalid"); e == nil {
		t.Fatal("missing provider")
	}
	if e := m.Finish(ctx, httptest.NewRecorder(), s, "spotify", "x", "code"); e == nil {
		t.Fatal("missing provider callback")
	}
	m.OAuth["spotify"] = &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.test/auth", TokenURL: "http://127.0.0.1:1"}}
	m.Providers["spotify"] = identityProvider{}
	link, e := m.Begin(ctx, s, "spotify")
	if e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(link)
	state := u.Query().Get("state")
	m.DB.Exec("UPDATE oauth_attempts SET expires_at=0")
	if e = m.Finish(ctx, httptest.NewRecorder(), s, "spotify", state, "code"); e == nil {
		t.Fatal("expired state")
	}
	link, _ = m.Begin(ctx, s, "spotify")
	u, _ = url.Parse(link)
	if e = m.Finish(ctx, httptest.NewRecorder(), s, "spotify", u.Query().Get("state"), "code"); e == nil {
		t.Fatal("exchange unexpectedly succeeded")
	}
	if e = m.Save(ctx, s.PrincipalID, "spotify", domain.Identity{}, domain.Token{}); e != ErrConnection {
		t.Fatal("empty identity")
	}
	m.Save(ctx, s.PrincipalID, "spotify", domain.Identity{ID: "x"}, domain.Token{})
	m.OAuth = nil
	if _, _, e = m.Token(ctx, s.PrincipalID, "spotify"); e != ErrConnection {
		t.Fatal("no refresh configured")
	}
}

type switchedProvider struct{ domain.Provider }

func (switchedProvider) Identity(context.Context, domain.Token) (domain.Identity, error) {
	return domain.Identity{ID: "other-account", Name: "Other"}, nil
}
func TestReconnectRejectsDifferentActiveIdentity(t *testing.T) {
	m := manager(t)
	ctx := context.Background()
	s, _ := session(t, m)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"new","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()
	m.OAuth["spotify"] = &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: server.URL + "/auth", TokenURL: server.URL + "/token"}}
	m.Providers["spotify"] = switchedProvider{}
	m.Active = func(context.Context, string) bool { return true }
	if e := m.Save(ctx, s.PrincipalID, "spotify", domain.Identity{ID: "original"}, domain.Token{AccessToken: "original-token", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}); e != nil {
		t.Fatal(e)
	}
	link, e := m.Begin(ctx, s, "spotify")
	if e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(link)
	if e = m.Finish(ctx, httptest.NewRecorder(), s, "spotify", u.Query().Get("state"), "code"); e == nil {
		t.Fatal("active identity replaced")
	}
	token, id, e := m.Token(ctx, s.PrincipalID, "spotify")
	if e != nil || id.ID != "original" || token.AccessToken != "original-token" {
		t.Fatal("connection changed", id, e)
	}
}
func TestDecodeAndDatabaseErrors(t *testing.T) {
	m := manager(t)
	if _, e := m.decode("p", "spotify", nil); e == nil {
		t.Fatal("invalid encrypted token accepted")
	}
	data, e := seal(m.Key, "p/spotify/v1", []byte("invalid json"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.decode("p", "spotify", data); e == nil {
		t.Fatal("bad json accepted")
	}
	m.DB.Close()
	if _, e = m.Connections(context.Background(), "p"); e == nil {
		t.Fatal("closed DB accepted")
	}
	if _, _, e = m.Token(context.Background(), "p", "spotify"); e == nil {
		t.Fatal("closed DB accepted")
	}
}
