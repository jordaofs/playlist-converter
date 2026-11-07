package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"playlistconverter/internal/auth"
	"playlistconverter/internal/config"
	"playlistconverter/internal/domain"
	"strings"
	"testing"
)

type fakeEngine struct{ calls int }

func (f *fakeEngine) Create(context.Context, string, string, string) (domain.Conversion, error) {
	f.calls++
	return domain.Conversion{ID: "job", Status: "queued"}, nil
}
func (f *fakeEngine) Get(context.Context, string, string) (domain.Conversion, error) {
	return domain.Conversion{ID: "job", Items: []domain.Item{}}, nil
}
func (f *fakeEngine) List(context.Context, string) ([]domain.Conversion, error) {
	return []domain.Conversion{}, nil
}
func (f *fakeEngine) Decide(context.Context, string, string, int, []domain.Decision) (domain.Conversion, error) {
	return domain.Conversion{ID: "job"}, nil
}
func (f *fakeEngine) Publish(context.Context, string, string, int, string, string) (domain.Conversion, error) {
	return domain.Conversion{ID: "job"}, nil
}
func (f *fakeEngine) Resume(context.Context, string, string) (domain.Conversion, error) {
	return domain.Conversion{ID: "job"}, nil
}
func (f *fakeEngine) Cancel(context.Context, string, string) (domain.Conversion, error) {
	return domain.Conversion{ID: "job"}, nil
}
func (f *fakeEngine) CancelPrincipal(context.Context, string) error { return nil }
func testServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	db, e := sql.Open("sqlite", ":memory:")
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, e = db.Exec(`PRAGMA foreign_keys=ON;CREATE TABLE principals(id TEXT PRIMARY KEY,created_at INTEGER,expires_at INTEGER);CREATE TABLE sessions(id TEXT PRIMARY KEY,principal_id TEXT REFERENCES principals(id) ON DELETE CASCADE,csrf TEXT,expires_at INTEGER);CREATE TABLE connections(principal_id TEXT REFERENCES principals(id) ON DELETE CASCADE,provider TEXT,identity_id TEXT,name TEXT,token BLOB,PRIMARY KEY(principal_id,provider));CREATE TABLE oauth_attempts(state TEXT PRIMARY KEY,principal_id TEXT,session_id TEXT,provider TEXT,verifier TEXT,expires_at INTEGER);`)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0600)
	s := &Server{Config: config.Config{BaseURL: "http://localhost", AssetsPath: dir}, Auth: &auth.Manager{DB: db, Key: make([]byte, 32), KeyID: "v1"}, Engine: &fakeEngine{}}
	return s, s.Handler()
}
func TestRouterBoundaries(t *testing.T) {
	_, h := testServer(t)
	for _, tc := range []struct {
		path   string
		status int
		kind   string
	}{{"/healthz", 200, "application/json"}, {"/readyz", 200, "application/json"}, {"/api/unknown", 404, "application/json"}, {"/some-page", 200, "text/html"}, {"/assets/missing.js", 404, "text/plain"}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || !strings.Contains(w.Header().Get("Content-Type"), tc.kind) {
			t.Errorf("%s got %d %s", tc.path, w.Code, w.Header().Get("Content-Type"))
		}
	}
}
func TestSessionAndCSRF(t *testing.T) {
	s, h := testServer(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/session", nil))
	if w.Code != 200 {
		t.Fatalf("session %d", w.Code)
	}
	var response struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &response)
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || response.Data.CSRFToken == "" {
		t.Fatal("missing session")
	}
	for _, tc := range []struct {
		csrf, origin, body string
		want               int
	}{{"", "http://localhost", `{"sourceUrl":"x"}`, 403}, {response.Data.CSRFToken, "http://evil.test", `{"sourceUrl":"x"}`, 403}, {response.Data.CSRFToken, "http://localhost", `{"sourceUrl":"x","unknown":1}`, 400}, {response.Data.CSRFToken, "http://localhost", `{"sourceUrl":"x"}`, 202}} {
		r := httptest.NewRequest("POST", "/api/conversions", strings.NewReader(tc.body))
		r.AddCookie(cookies[0])
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", tc.csrf)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Idempotency-Key", "key")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
		}
	}
	if s.Engine.(*fakeEngine).calls != 1 {
		t.Fatal("unprotected mutation")
	}
}
