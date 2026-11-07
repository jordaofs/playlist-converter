package httpapi

import (
	"context"
	"net/http"
	"playlistconverter/internal/auth"
	"playlistconverter/internal/config"
	"playlistconverter/internal/domain"
)

type Engine interface {
	Create(context.Context, string, string, string) (domain.Conversion, error)
	Get(context.Context, string, string) (domain.Conversion, error)
	List(context.Context, string) ([]domain.Conversion, error)
	Decide(context.Context, string, string, int, []domain.Decision) (domain.Conversion, error)
	Publish(context.Context, string, string, int, string, string) (domain.Conversion, error)
	Resume(context.Context, string, string) (domain.Conversion, error)
	Cancel(context.Context, string, string) (domain.Conversion, error)
	CancelPrincipal(context.Context, string) error
}
type Server struct {
	Config config.Config
	Auth   *auth.Manager
	Engine Engine
	// RateLimitBuckets caps distinct clients tracked per minute (default 10000).
	RateLimitBuckets int
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { reply(w, r, 200, map[string]string{"status": "ok"}, "") })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if e := s.Auth.DB.PingContext(r.Context()); e != nil {
			reply(w, r, 503, nil, "database_unavailable")
			return
		}
		reply(w, r, 200, map[string]string{"status": "ready"}, "")
	})
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("DELETE /api/session", s.withSession(s.deleteSession, true))
	mux.HandleFunc("GET /api/auth/{provider}/start", s.withSession(s.beginAuth, false))
	mux.HandleFunc("GET /api/auth/{provider}/callback", s.withSession(s.finishAuth, false))
	mux.HandleFunc("DELETE /api/connections/{provider}", s.withSession(s.disconnect, true))
	mux.HandleFunc("POST /api/conversions", s.withSession(s.create, true))
	mux.HandleFunc("GET /api/conversions/{id}", s.withSession(s.get, false))
	mux.HandleFunc("GET /api/conversions/{id}/items", s.withSession(s.items, false))
	mux.HandleFunc("PATCH /api/conversions/{id}/decisions", s.withSession(s.decide, true))
	mux.HandleFunc("POST /api/conversions/{id}/publish", s.withSession(s.publish, true))
	mux.HandleFunc("POST /api/conversions/{id}/resume", s.withSession(s.resume, true))
	mux.HandleFunc("POST /api/conversions/{id}/cancel", s.withSession(s.cancel, true))
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) { reply(w, r, 404, nil, "not_found") })
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { reply(w, r, 404, nil, "not_found") })
	mux.HandleFunc("/", s.static)
	return s.middleware(mux)
}
