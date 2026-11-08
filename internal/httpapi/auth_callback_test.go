package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"playlistconverter/internal/domain"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/oauth2"
)

type noChannel struct{ domain.Provider }

func (noChannel) Identity(context.Context, domain.Token) (domain.Identity, error) {
	return domain.Identity{}, &domain.ProviderError{Code: "channel_selection_required"}
}

func logLines(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n")) {
		if len(raw) == 0 {
			continue
		}
		var line map[string]any
		if e := json.Unmarshal(raw, &line); e != nil {
			t.Fatal(e)
		}
		lines = append(lines, line)
	}
	return lines
}

// Failed OAuth callbacks are logged with closed codes only, and a missing
// YouTube channel gets its own message instead of a generic failure.
func TestFailedCallbackIsLoggedAndExplained(t *testing.T) {
	s, _ := testServer(t)
	var out bytes.Buffer
	s.Log = slog.New(slog.NewJSONHandler(&out, nil))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"access-do-not-log","token_type":"Bearer","expires_in":3600}`)
	}))
	defer ts.Close()
	s.Auth.Client = ts.Client()
	s.Auth.OAuth = map[string]*oauth2.Config{"youtube": {ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token"}}}
	s.Auth.Providers = map[string]domain.Provider{"youtube": noChannel{}}
	h := s.Handler()
	cookie, _ := authenticated(t, s)
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(cookie)
		r.Header.Set("Referer", s.Config.BaseURL+"/")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	start := get("/api/auth/youtube/start")
	u, e := url.Parse(start.Header().Get("Location"))
	if start.Code != 302 || e != nil {
		t.Fatal(start.Code, e)
	}
	state := u.Query().Get("state")

	w := get("/api/auth/youtube/callback?state=" + url.QueryEscape(state) + "&code=code-do-not-log")
	if w.Code != 303 || w.Header().Get("Location") != "/?auth=no_channel" {
		t.Fatal(w.Code, w.Header().Get("Location"))
	}
	w = get("/api/auth/youtube/callback?state=stale&code=code-do-not-log")
	if w.Code != 303 || w.Header().Get("Location") != "/?auth=failed" {
		t.Fatal(w.Code, w.Header().Get("Location"))
	}

	lines := logLines(t, &out)
	if len(lines) != 2 {
		t.Fatalf("want 2 log lines, got %v", lines)
	}
	want := []map[string]string{
		{"msg": "auth.failed", "provider": "youtube", "reason": "provider_identity_failed", "cause": "channel_selection_required"},
		{"msg": "auth.failed", "provider": "youtube", "reason": "invalid_oauth_state"},
	}
	for i, fields := range want {
		for k, v := range fields {
			if lines[i][k] != v {
				t.Fatalf("line %d: %s = %v, want %s (%v)", i, k, lines[i][k], v, lines[i])
			}
		}
		for k := range lines[i] {
			switch k {
			case "time", "level", "msg", "provider", "reason", "cause":
			default:
				t.Fatalf("unexpected log field %q", k)
			}
		}
	}
	for _, secret := range []string{"code-do-not-log", "access-do-not-log", state, "stale"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("log contains %q", secret)
		}
	}
}

func TestCallbackLogSanitizesProvider(t *testing.T) {
	s, h := testServer(t)
	var out bytes.Buffer
	s.Log = slog.New(slog.NewJSONHandler(&out, nil))
	cookie, _ := authenticated(t, s)
	r := httptest.NewRequest("GET", "/api/auth/Evil-Provider/callback?state=x&code=y", nil)
	r.AddCookie(cookie)
	r.Header.Set("Referer", s.Config.BaseURL+"/")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	lines := logLines(t, &out)
	if len(lines) != 1 || lines[0]["provider"] != "other" || strings.Contains(out.String(), "Evil") {
		t.Fatalf("%v", lines)
	}
}

type okIdentity struct{ domain.Provider }

func (okIdentity) Identity(context.Context, domain.Token) (domain.Identity, error) {
	return domain.Identity{ID: "channel", Name: "Me"}, nil
}

// Regression (production, 2026-11-08): Google's callback reached the server
// twice. The first delivery connected the account and rotated the session;
// the second, still carrying the old cookie, overwrote the page with
// "We couldn't connect your account".
func TestDuplicateCallbackConnectsOnce(t *testing.T) {
	s, _ := testServer(t)
	var out bytes.Buffer
	s.Log = slog.New(slog.NewJSONHandler(&out, nil))
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"a","refresh_token":"r","token_type":"Bearer","expires_in":3600}`)
	}))
	defer ts.Close()
	s.Auth.Client = ts.Client()
	s.Auth.OAuth = map[string]*oauth2.Config{"youtube": {ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token"}}}
	s.Auth.Providers = map[string]domain.Provider{"youtube": okIdentity{}}
	h := s.Handler()
	cookie, _ := authenticated(t, s)
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(cookie)
		r.Header.Set("Referer", s.Config.BaseURL+"/")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	u, _ := url.Parse(get("/api/auth/youtube/start").Header().Get("Location"))
	callback := "/api/auth/youtube/callback?state=" + url.QueryEscape(u.Query().Get("state")) + "&code=once"
	kept := func(w *httptest.ResponseRecorder) string {
		value := ""
		for _, c := range w.Result().Cookies() {
			if c.Name == cookie.Name {
				value = c.Value
			}
		}
		return value
	}
	first, second := get(callback), get(callback)
	for i, w := range []*httptest.ResponseRecorder{first, second} {
		if w.Code != 303 || w.Header().Get("Location") != "/?auth=connected" {
			t.Fatalf("delivery %d: %d %s", i+1, w.Code, w.Header().Get("Location"))
		}
	}
	if kept(first) == "" || kept(first) == cookie.Value || kept(first) != kept(second) {
		t.Fatal("both deliveries must leave the browser with the same rotated session")
	}
	if calls.Load() != 1 {
		t.Fatalf("token exchanges = %d", calls.Load())
	}
	if lines := logLines(t, &out); len(lines) != 0 {
		t.Fatalf("unexpected log lines %v", lines)
	}
}
