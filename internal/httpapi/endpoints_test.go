package httpapi

import (
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"playlistconverter/internal/auth"
	"playlistconverter/internal/conversions"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"strings"
	"testing"
	"time"
)

func authenticated(t *testing.T, s *Server) (*http.Cookie, string) {
	t.Helper()
	w := httptest.NewRecorder()
	v, e := s.Auth.Session(context.Background(), w, httptest.NewRequest("GET", "/", nil), true)
	if e != nil {
		t.Fatal(e)
	}
	return w.Result().Cookies()[0], v.CSRF
}
func TestAuthenticatedEndpoints(t *testing.T) {
	s, h := testServer(t)
	cookie, csrf := authenticated(t, s)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/conversions/job", "", 200}, {"GET", "/api/conversions/job/items", "", 200}, {"GET", "/api/conversions/job/items?cursor=no", "", 400}, {"GET", "/api/conversions/job/items?cursor=3", "", 400}, {"GET", "/api/conversions/job/items?limit=0", "", 400}, {"GET", "/api/conversions/job/items?limit=1", "", 200}, {"PATCH", "/api/conversions/job/decisions", `{"revision":1,"decisions":[]}`, 200}, {"POST", "/api/conversions/job/publish", `{"revision":1,"name":"List"}`, 202}, {"POST", "/api/conversions/job/resume", "", 202}, {"POST", "/api/conversions/job/cancel", "", 200}, {"DELETE", "/api/connections/invalid", "", 400}, {"DELETE", "/api/connections/spotify", "", 200}, {"GET", "/api/auth/spotify/start", "", 400}, {"GET", "/api/auth/spotify/callback?error=access_denied", "", 303}, {"GET", "/api/auth/spotify/callback?state=bad&code=bad", "", 303}, {"POST", "/api/conversions", `{} {}`, 400}, {"PATCH", "/api/conversions/job/decisions", `{"bad":1}`, 400}, {"POST", "/api/conversions/job/publish", `{"bad":1}`, 400}, {"DELETE", "/api/session", "", 200}} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.AddCookie(cookie)
		r.Header.Set("Origin", s.Config.BaseURL)
		r.Header.Set("Referer", s.Config.BaseURL+"/")
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
func TestRequestAndStaticErrors(t *testing.T) {
	s, h := testServer(t)
	cookie, csrf := authenticated(t, s)
	for _, tc := range []struct {
		path, method, body, content string
		cookie                      bool
		want                        int
	}{{"/api/conversions", "POST", "{}", "", false, 401}, {"/api/conversions", "POST", "{}", "text/plain", true, 415}, {"/api/conversions", "POST", strings.Repeat("x", 40000), "application/json", true, 400}, {"/api/auth/spotify/start", "GET", "", "", true, 403}, {"/", "POST", "", "", false, 405}, {"/api", "GET", "", "", false, 404}} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.cookie {
			r.AddCookie(cookie)
		}
		r.Header.Set("Origin", s.Config.BaseURL)
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Content-Type", tc.content)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	os.Remove(s.Config.AssetsPath + "/index.html")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	s.Auth.DB.Close()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestRateLimit(t *testing.T) {
	_, h := testServer(t)
	var w *httptest.ResponseRecorder
	for i := 0; i < 241; i++ {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	}
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("not rate limited")
	}
}

// Regression: behind the host reverse proxy every request arrives from the same
// peer, so the limit must apply to the forwarded client rather than the proxy.
func TestRateLimitBehindTrustedProxy(t *testing.T) {
	s, _ := testServer(t)
	s.Config.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}
	h := s.Handler()
	request := func(client string) int {
		r := httptest.NewRequest("GET", "/healthz", nil)
		r.RemoteAddr = "172.18.0.1:40000"
		r.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 240; i++ {
		if code := request("198.51.100.1"); code != 200 {
			t.Fatalf("request %d limited early: %d", i, code)
		}
	}
	if code := request("198.51.100.1"); code != 429 {
		t.Fatalf("noisy client not limited: %d", code)
	}
	if code := request("198.51.100.2"); code != 200 {
		t.Fatalf("other client shares the proxy bucket: %d", code)
	}
}
func TestProblem(t *testing.T) {
	for _, e := range []error{&conversions.Error{Code: "not_found", Status: 404}, auth.ErrConnection} {
		w := httptest.NewRecorder()
		problem(w, httptest.NewRequest("GET", "/", nil), e)
		if w.Code != 404 && w.Code != 500 {
			t.Fatal(w.Code)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestQuotaBeforeDispatch(t *testing.T) {
	db, e := store.Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	calls := 0
	q := &QuotaTransport{Store: db, SearchLimit: 1, UnitLimit: 50, Location: time.UTC, Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})}
	for _, path := range []string{"/youtube/v3/search", "/youtube/v3/search"} {
		r := httptest.NewRequest("GET", path, nil)
		_, e = q.RoundTrip(r)
	}
	if e == nil || calls != 1 {
		t.Fatal("search budget not enforced")
	}
	r := httptest.NewRequest("POST", "/youtube/v3/playlists", nil)
	if _, e = q.RoundTrip(r); e != nil {
		t.Fatal(e)
	}
	_, e = q.RoundTrip(r)
	if e == nil || calls != 2 {
		t.Fatal("write budget not enforced")
	}
	r = httptest.NewRequest("GET", "/v1/me", nil)
	if _, e = q.RoundTrip(r); e != nil {
		t.Fatal(e)
	}
}
func TestBeginOAuth(t *testing.T) {
	s, h := testServer(t)
	s.Auth.OAuth = map[string]*oauth2.Config{"spotify": {ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example/auth"}}}
	cookie, _ := authenticated(t, s)
	r := httptest.NewRequest("GET", "/api/auth/spotify/start", nil)
	r.AddCookie(cookie)
	r.Header.Set("Referer", s.Config.BaseURL+"/")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 302 || !strings.HasPrefix(w.Header().Get("Location"), "https://accounts.example/auth") {
		t.Fatal(w.Code, w.Body.String())
	}
}

var _ = domain.Token{}

// Regression: with real client addresses the bucket table can fill up. A full
// table must not grow, must not lock out clients that already have a bucket,
// and must only refuse new clients.
func TestRateLimitTableIsBounded(t *testing.T) {
	s, _ := testServer(t)
	s.RateLimitBuckets = 50
	h := s.Handler()
	request := func(ip string) int {
		r := httptest.NewRequest("GET", "/healthz", nil)
		r.RemoteAddr = ip + ":1000"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 50; i++ {
		if code := request(fmt.Sprintf("198.51.100.%d", i)); code != 200 {
			t.Fatalf("client %d refused while table had room: %d", i, code)
		}
	}
	if code := request("203.0.113.1"); code != 429 {
		t.Fatalf("new client admitted into a full table: %d", code)
	}
	if code := request("198.51.100.0"); code != 200 {
		t.Fatalf("existing client locked out by a full table: %d", code)
	}
}
