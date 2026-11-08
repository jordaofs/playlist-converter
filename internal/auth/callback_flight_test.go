package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// Production evidence (2026-11-08): browsers delivered Google's callback twice
// and sometimes aborted the first request. Every duplicate failed on the used
// state or the rotated cookie, so a working connection looked broken.

type tokenServer struct {
	calls   atomic.Int32
	arrived chan struct{}
	release chan struct{}
}

// flow prepares a YouTube attempt. With block set, the token endpoint waits
// for release, so tests can line up duplicates while the exchange is running.
func flow(t *testing.T, m *Manager, block bool) (*tokenServer, *http.Cookie, string) {
	t.Helper()
	ts := &tokenServer{arrived: make(chan struct{}, 8), release: make(chan struct{})}
	if !block {
		close(ts.release)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.calls.Add(1)
		ts.arrived <- struct{}{}
		<-ts.release
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"a","refresh_token":"r","token_type":"Bearer","expires_in":3600}`)
	}))
	t.Cleanup(srv.Close)
	m.Client = srv.Client()
	m.OAuth["youtube"] = &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: srv.URL + "/auth", TokenURL: srv.URL + "/token"}}
	if m.Providers["youtube"] == nil {
		m.Providers["youtube"] = identityProvider{}
	}
	s, cookie := session(t, m)
	link, e := m.Begin(context.Background(), s, "youtube")
	if e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(link)
	return ts, cookie, u.Query().Get("state")
}

func callbackRequest(ctx context.Context, state string, cookie *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", "/api/auth/youtube/callback?state="+url.QueryEscape(state)+"&code=c", nil).WithContext(ctx)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	return r
}

// sessionCookie returns the session cookie the browser keeps: the last one
// set. The owner response first refreshes the old cookie, then rotates it.
func sessionCookie(w *httptest.ResponseRecorder) string {
	value := ""
	for _, c := range w.Result().Cookies() {
		if c.Name == "converter_session" {
			value = c.Value
		}
	}
	return value
}

func callback(m *Manager, ctx context.Context, state string, cookie *http.Cookie) (*httptest.ResponseRecorder, bool, error) {
	w := httptest.NewRecorder()
	replayed, e := m.Callback(ctx, w, callbackRequest(ctx, state, cookie), "youtube", state, "c")
	return w, replayed, e
}

func connections(t *testing.T, m *Manager) int {
	t.Helper()
	var n int
	if e := m.DB.QueryRow("SELECT COUNT(*) FROM connections WHERE provider='youtube'").Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestCallbackConcurrentDuplicates(t *testing.T) {
	m := manager(t)
	ts, cookie, state := flow(t, m, true)
	type result struct {
		w        *httptest.ResponseRecorder
		replayed bool
		err      error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	run := func() {
		defer wg.Done()
		w, replayed, e := callback(m, context.Background(), state, cookie)
		results <- result{w, replayed, e}
	}
	wg.Add(2)
	go run()
	<-ts.arrived // the first request is inside the token exchange
	go run()
	time.Sleep(20 * time.Millisecond)
	close(ts.release)
	wg.Wait()
	close(results)
	replays, values := 0, map[string]bool{}
	for r := range results {
		if r.err != nil {
			t.Fatalf("duplicate failed: %v", r.err)
		}
		if r.replayed {
			replays++
		}
		values[sessionCookie(r.w)] = true
	}
	if replays != 1 || ts.calls.Load() != 1 || len(values) != 1 || values[""] {
		t.Fatalf("replays=%d calls=%d cookies=%v", replays, ts.calls.Load(), values)
	}
	if connections(t, m) != 1 {
		t.Fatal("connection not saved once")
	}
}

func TestCallbackLateDuplicateWithOldCookie(t *testing.T) {
	m := manager(t)
	ts, cookie, state := flow(t, m, false)
	first, replayed, e := callback(m, context.Background(), state, cookie)
	if e != nil || replayed {
		t.Fatal(replayed, e)
	}
	second, replayed, e := callback(m, context.Background(), state, cookie)
	if e != nil || !replayed {
		t.Fatal(replayed, e)
	}
	if sessionCookie(first) == "" || sessionCookie(first) != sessionCookie(second) || ts.calls.Load() != 1 {
		t.Fatalf("cookies %q %q calls %d", sessionCookie(first), sessionCookie(second), ts.calls.Load())
	}
}

func TestCallbackSurvivesAbortedRequest(t *testing.T) {
	m := manager(t)
	ts, cookie, state := flow(t, m, true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, e := callback(m, ctx, state, cookie)
		done <- e
	}()
	<-ts.arrived
	cancel() // the browser drops the first request mid-exchange
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatalf("owner returned %v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("aborted owner kept waiting")
	}
	close(ts.release)
	w, replayed, e := callback(m, context.Background(), state, cookie)
	if e != nil || !replayed || sessionCookie(w) == "" {
		t.Fatal(replayed, e)
	}
	if connections(t, m) != 1 || ts.calls.Load() != 1 {
		t.Fatal("exchange was not completed exactly once")
	}
}

func TestCallbackDuplicateFromOtherSessionFails(t *testing.T) {
	m := manager(t)
	_, cookie, state := flow(t, m, false)
	if _, _, e := callback(m, context.Background(), state, cookie); e != nil {
		t.Fatal(e)
	}
	_, other := session(t, m)
	w, replayed, e := callback(m, context.Background(), state, other)
	var ce *CallbackError
	if !errors.As(e, &ce) || ce.Code != "invalid_oauth_state" || replayed {
		t.Fatalf("got %v replayed=%v", e, replayed)
	}
	if sessionCookie(w) != "" {
		t.Fatal("foreign session received a cookie")
	}
}

func TestCallbackFailureReplayedOnce(t *testing.T) {
	m := manager(t)
	m.Providers["youtube"] = failingIdentity{}
	_, cookie, state := flow(t, m, false)
	for i, wantReplay := range []bool{false, true} {
		w, replayed, e := callback(m, context.Background(), state, cookie)
		var ce *CallbackError
		if !errors.As(e, &ce) || ce.Code != "provider_identity_failed" || ce.Cause != "channel_selection_required" || replayed != wantReplay {
			t.Fatalf("call %d: %v replayed=%v", i, e, replayed)
		}
		if sessionCookie(w) != "" && i == 1 {
			t.Fatal("failed replay set a new session cookie")
		}
	}
}

func TestCallbackFlightsExpire(t *testing.T) {
	m := manager(t)
	m.CallbackTTL = time.Millisecond
	_, cookie, state := flow(t, m, false)
	first, _, e := callback(m, context.Background(), state, cookie)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(5 * time.Millisecond)
	fresh := &http.Cookie{Name: "converter_session", Value: sessionCookie(first)}
	_, replayed, e := callback(m, context.Background(), state, fresh)
	var ce *CallbackError
	if !errors.As(e, &ce) || ce.Code != "invalid_oauth_state" || replayed {
		t.Fatalf("expired flight reused: %v replayed=%v", e, replayed)
	}
	time.Sleep(5 * time.Millisecond)
	_, cookie2, state2 := flow(t, m, false)
	if _, _, e = callback(m, context.Background(), state2, cookie2); e != nil {
		t.Fatal(e)
	}
	m.flightMu.Lock()
	defer m.flightMu.Unlock()
	if _, ok := m.flights[hash(state)]; ok || len(m.flights) != 1 {
		t.Fatalf("expired flights kept: %d", len(m.flights))
	}
}

func TestCallbackWithoutSession(t *testing.T) {
	m := manager(t)
	_, _, state := flow(t, m, false)
	if _, _, e := callback(m, context.Background(), state, nil); !errors.Is(e, ErrSession) {
		t.Fatalf("got %v", e)
	}
}
