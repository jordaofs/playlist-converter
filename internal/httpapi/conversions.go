package httpapi

import (
	"net/http"
	"playlistconverter/internal/auth"
	"playlistconverter/internal/domain"
	"strconv"
)

func (s *Server) create(w http.ResponseWriter, r *http.Request, v auth.Session) {
	var body struct {
		SourceURL string `json:"sourceUrl"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, e := s.Engine.Create(r.Context(), v.PrincipalID, body.SourceURL, r.Header.Get("Idempotency-Key"))
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 202, c, "")
}
func (s *Server) get(w http.ResponseWriter, r *http.Request, v auth.Session) {
	c, e := s.Engine.Get(r.Context(), v.PrincipalID, r.PathValue("id"))
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 200, c, "")
}
func (s *Server) items(w http.ResponseWriter, r *http.Request, v auth.Session) {
	c, e := s.Engine.Get(r.Context(), v.PrincipalID, r.PathValue("id"))
	if e != nil {
		problem(w, r, e)
		return
	}
	offset, limit := 0, 50
	if x := r.URL.Query().Get("cursor"); x != "" {
		offset, e = strconv.Atoi(x)
		if e != nil || offset < 0 || offset > len(c.Items) {
			reply(w, r, 400, nil, "invalid_cursor")
			return
		}
	}
	if x := r.URL.Query().Get("limit"); x != "" {
		limit, e = strconv.Atoi(x)
		if e != nil || limit < 1 || limit > 100 {
			reply(w, r, 400, nil, "invalid_limit")
			return
		}
	}
	end := min(offset+limit, len(c.Items))
	next := ""
	if end < len(c.Items) {
		next = strconv.Itoa(end)
	}
	reply(w, r, 200, map[string]any{"items": c.Items[offset:end], "nextCursor": next}, "")
}
func (s *Server) decide(w http.ResponseWriter, r *http.Request, v auth.Session) {
	var body struct {
		Revision  int               `json:"revision"`
		Decisions []domain.Decision `json:"decisions"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, e := s.Engine.Decide(r.Context(), v.PrincipalID, r.PathValue("id"), body.Revision, body.Decisions)
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 200, c, "")
}
func (s *Server) publish(w http.ResponseWriter, r *http.Request, v auth.Session) {
	var body struct {
		Revision int    `json:"revision"`
		Name     string `json:"name"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, e := s.Engine.Publish(r.Context(), v.PrincipalID, r.PathValue("id"), body.Revision, body.Name, r.Header.Get("Idempotency-Key"))
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 202, c, "")
}
func (s *Server) resume(w http.ResponseWriter, r *http.Request, v auth.Session) {
	c, e := s.Engine.Resume(r.Context(), v.PrincipalID, r.PathValue("id"))
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 202, c, "")
}
func (s *Server) cancel(w http.ResponseWriter, r *http.Request, v auth.Session) {
	c, e := s.Engine.Cancel(r.Context(), v.PrincipalID, r.PathValue("id"))
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 200, c, "")
}
