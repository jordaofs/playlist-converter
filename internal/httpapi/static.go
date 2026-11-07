package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		reply(w, r, 405, nil, "method_not_allowed")
		return
	}
	if r.URL.Path == "/api" {
		reply(w, r, 404, nil, "not_found")
		return
	}
	if r.URL.Path == "/privacy" {
		policy := filepath.Join(s.Config.AssetsPath, "privacy.html")
		if info, err := os.Stat(policy); err != nil || !info.Mode().IsRegular() {
			reply(w, r, 503, nil, "privacy_document_unavailable")
			return
		}
		http.ServeFile(w, r, policy)
		return
	}
	clean := filepath.Clean("/" + r.URL.Path)
	path := filepath.Join(s.Config.AssetsPath, clean)
	info, e := os.Stat(path)
	if e == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	if filepath.Ext(clean) != "" || strings.HasPrefix(clean, "/assets/") {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(s.Config.AssetsPath, "index.html")
	if _, e = os.Stat(index); e != nil {
		reply(w, r, 503, nil, "frontend_not_built")
		return
	}
	http.ServeFile(w, r, index)
}
