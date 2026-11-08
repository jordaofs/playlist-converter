package auth

import (
	"context"
	"net/http"
	"time"
)

// Browsers sometimes deliver an OAuth callback twice and abort the first
// request (seen with Google in production). The state and code are single use
// and a successful callback rotates the session, so every duplicate used to
// fail and hide a working connection. A flight runs the callback once per
// state and hands its outcome to duplicates from the same browser.
type flight struct {
	session string // hash of the session cookie that started the callback
	done    chan struct{}
	raw     string // new session token, set on success before done closes
	err     error
	expires time.Time
}

const callbackTimeout = 30 * time.Second

func (m *Manager) callbackTTL() time.Duration {
	if m.CallbackTTL > 0 {
		return m.CallbackTTL
	}
	return 2 * time.Minute
}

// Callback finishes an OAuth callback once per state. The exchange runs
// detached from the request, so it completes even if the browser drops the
// connection. Duplicates presenting the cookie that started it, or the new
// cookie it issued, wait for or reuse the outcome; replayed reports that case
// so callers log each failure once. Other sessions get invalid_oauth_state.
func (m *Manager) Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, provider, state, code string) (bool, error) {
	key := hash(state)
	cookie := ""
	if c, e := r.Cookie("converter_session"); e == nil {
		cookie = hash(c.Value)
	}
	if f := m.flight(key); f != nil {
		return m.join(ctx, w, f, cookie)
	}
	s, e := m.Session(ctx, w, r, false)
	if e != nil {
		return false, ErrSession
	}
	f, owner := m.register(key, s.ID)
	if !owner {
		return m.join(ctx, w, f, cookie)
	}
	go func() {
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), callbackTimeout)
		defer cancel()
		raw, err := m.complete(work, s, provider, state, code)
		m.flightMu.Lock()
		f.raw, f.err = raw, err
		f.expires = time.Now().Add(m.callbackTTL())
		m.flightMu.Unlock()
		close(f.done)
	}()
	select {
	case <-f.done:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return false, m.outcome(w, f)
}

// flight returns the live flight for a state, if any.
func (m *Manager) flight(key string) *flight {
	m.flightMu.Lock()
	defer m.flightMu.Unlock()
	f := m.flights[key]
	if f == nil || m.expired(f) {
		return nil
	}
	return f
}

// register adds a flight for the state, or returns the one another request
// registered first. Expired flights are pruned on every insert.
func (m *Manager) register(key, session string) (*flight, bool) {
	m.flightMu.Lock()
	defer m.flightMu.Unlock()
	if f := m.flights[key]; f != nil && !m.expired(f) {
		return f, false
	}
	if m.flights == nil {
		m.flights = map[string]*flight{}
	}
	for k, f := range m.flights {
		if m.expired(f) {
			delete(m.flights, k)
		}
	}
	f := &flight{session: session, done: make(chan struct{})}
	m.flights[key] = f
	return f, true
}

// expired reports whether a finished flight is past its TTL. Running flights
// never expire; the detached exchange is bounded by callbackTimeout.
// The caller holds flightMu.
func (m *Manager) expired(f *flight) bool {
	return !f.expires.IsZero() && time.Now().After(f.expires)
}

// join waits for a flight started by the same browser and returns its
// outcome as a replay. Another browser is refused like a used state.
func (m *Manager) join(ctx context.Context, w http.ResponseWriter, f *flight, cookie string) (bool, error) {
	if !m.sameBrowser(f, cookie) {
		return false, &CallbackError{Code: "invalid_oauth_state"}
	}
	select {
	case <-f.done:
	case <-ctx.Done():
		return true, ctx.Err()
	}
	return true, m.outcome(w, f)
}

// sameBrowser accepts the cookie that started the flight or, once it
// succeeded, the cookie it issued. Before done closes only the first applies.
func (m *Manager) sameBrowser(f *flight, cookie string) bool {
	if cookie == "" {
		return false
	}
	if cookie == f.session {
		return true
	}
	m.flightMu.Lock()
	defer m.flightMu.Unlock()
	return f.raw != "" && hash(f.raw) == cookie
}

// outcome applies a finished flight to the response. The caller has seen
// done closed, so raw and err are final.
func (m *Manager) outcome(w http.ResponseWriter, f *flight) error {
	m.flightMu.Lock()
	raw, err := f.raw, f.err
	m.flightMu.Unlock()
	if err != nil {
		return err
	}
	m.cookie(w, raw, int(lifetime.Seconds()))
	return nil
}
