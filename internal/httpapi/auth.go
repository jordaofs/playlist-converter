package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"playlistconverter/internal/auth"
	"regexp"
)

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	v, e := s.Auth.Session(r.Context(), w, r, true)
	if e != nil {
		problem(w, r, e)
		return
	}
	connections, e := s.Auth.Connections(r.Context(), v.PrincipalID)
	if e != nil {
		problem(w, r, e)
		return
	}
	jobs, e := s.Engine.List(r.Context(), v.PrincipalID)
	if e != nil {
		problem(w, r, e)
		return
	}
	ids := []string{}
	for _, j := range jobs {
		ids = append(ids, j.ID)
	}
	reply(w, r, 200, map[string]any{"csrfToken": v.CSRF, "connections": connections, "limits": map[string]int{"maxItems": 100}, "conversionIds": ids}, "")
}
func (s *Server) beginAuth(w http.ResponseWriter, r *http.Request, v auth.Session) {
	ref, e := url.Parse(r.Referer())
	if e != nil || ref.Scheme+"://"+ref.Host != s.Config.BaseURL {
		reply(w, r, 403, nil, "same_origin_required")
		return
	}
	link, e := s.Auth.Begin(r.Context(), v, r.PathValue("provider"))
	if e != nil {
		reply(w, r, 400, nil, "provider_not_configured")
		return
	}
	http.Redirect(w, r, link, http.StatusFound)
}
func (s *Server) finishAuth(w http.ResponseWriter, r *http.Request, v auth.Session) {
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/?auth=denied", http.StatusSeeOther)
		return
	}
	provider := r.PathValue("provider")
	if e := s.Auth.Finish(r.Context(), w, v, provider, r.URL.Query().Get("state"), r.URL.Query().Get("code")); e != nil {
		reason, cause := "internal", ""
		var ce *auth.CallbackError
		if errors.As(e, &ce) {
			reason, cause = ce.Code, ce.Cause
		}
		s.logAuthFailure(provider, reason, cause)
		if cause == "channel_selection_required" {
			http.Redirect(w, r, "/?auth=no_channel", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?auth=failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?auth=connected", http.StatusSeeOther)
}
func (s *Server) disconnect(w http.ResponseWriter, r *http.Request, v auth.Session) {
	provider := r.PathValue("provider")
	if provider != "spotify" && provider != "youtube" {
		reply(w, r, 400, nil, "invalid_provider")
		return
	}
	if e := s.Engine.CancelPrincipal(r.Context(), v.PrincipalID); e != nil {
		problem(w, r, e)
		return
	}
	_, e := s.Auth.DB.ExecContext(r.Context(), "DELETE FROM connections WHERE principal_id=? AND provider=?", v.PrincipalID, provider)
	if e != nil {
		problem(w, r, e)
		return
	}
	reply(w, r, 200, map[string]bool{"disconnected": true}, "")
}
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request, v auth.Session) {
	if e := s.Engine.CancelPrincipal(r.Context(), v.PrincipalID); e != nil {
		problem(w, r, e)
		return
	}
	_, e := s.Auth.DB.ExecContext(r.Context(), "DELETE FROM principals WHERE id=?", v.PrincipalID)
	if e != nil {
		problem(w, r, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "converter_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Auth.Secure, SameSite: http.SameSiteLaxMode})
	reply(w, r, 200, map[string]bool{"deleted": true}, "")
}

var logCode = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// logAuthFailure records a failed OAuth callback with a closed field set:
// provider (spotify, youtube or other), the failed step and the provider's
// error code. Codes that are not short identifiers become "other".
func (s *Server) logAuthFailure(provider, reason, cause string) {
	if provider != "spotify" && provider != "youtube" {
		provider = "other"
	}
	if !logCode.MatchString(reason) {
		reason = "other"
	}
	attrs := []any{"provider", provider, "reason", reason}
	if cause != "" {
		if !logCode.MatchString(cause) {
			cause = "other"
		}
		attrs = append(attrs, "cause", cause)
	}
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	log.Warn("auth.failed", attrs...)
}
