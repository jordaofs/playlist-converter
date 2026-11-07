package httpapi

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivacyDocument(t *testing.T) {
	s, h := testServer(t)
	content := "<!doctype html><title>Privacy</title><p>Tokens are encrypted.</p>"
	if e := os.WriteFile(filepath.Join(s.Config.AssetsPath, "privacy.html"), []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/privacy", nil))
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Fatal(w.Code, w.Header())
		}
		if method == "GET" && w.Body.String() != content {
			t.Fatal("privacy route returned SPA instead of document")
		}
	}
	os.Remove(filepath.Join(s.Config.AssetsPath, "privacy.html"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/privacy", nil))
	if w.Code != 503 {
		t.Fatalf("missing policy must not return SPA: %d", w.Code)
	}
}

// Crawler files generated at build time must be served as files, never as the
// SPA shell, and must not exist as SPA fallbacks when absent.
func TestCrawlerFiles(t *testing.T) {
	s, h := testServer(t)
	files := map[string]string{"robots.txt": "User-agent: *\nDisallow: /api/\n", "sitemap.xml": "<?xml version=\"1.0\"?><urlset/>"}
	for name, content := range files {
		if e := os.WriteFile(filepath.Join(s.Config.AssetsPath, name), []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/"+name, nil))
		if w.Code != 200 || w.Body.String() != content {
			t.Fatalf("%s: %d %q", name, w.Code, w.Body.String())
		}
	}
	os.Remove(filepath.Join(s.Config.AssetsPath, "sitemap.xml"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if w.Code != 404 {
		t.Fatalf("missing sitemap must not return the SPA: %d", w.Code)
	}
}
