package httpapi

import (
	"crypto/subtle"
	"net/http"
	"playlistconverter/internal/auth"
	"sync"
	"time"
)

type sessionHandler func(http.ResponseWriter, *http.Request, auth.Session)

func (s *Server) withSession(fn sessionHandler, mutation bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Auth.Session(r.Context(), w, r, false)
		if e != nil {
			reply(w, r, 401, nil, "session_required")
			return
		}
		if mutation && (r.Header.Get("Origin") != s.Config.BaseURL || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(v.CSRF)) != 1) {
			reply(w, r, 403, nil, "csrf_failed")
			return
		}
		fn(w, r, v)
	}
}
func (s *Server) middleware(next http.Handler) http.Handler {
	type entry struct {
		count int
		until time.Time
	}
	var mu sync.Mutex
	buckets := map[string]entry{}
	limit := s.RateLimitBuckets
	if limit <= 0 {
		limit = 10000
	}
	var swept time.Time
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", auth.Random())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if s.Auth.Secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		host := rateKey(clientIP(r, s.Config.TrustedProxies))
		now := time.Now()
		mu.Lock()
		// Sweep at most once per second instead of scanning the table on every
		// request; expired buckets still reset below when they are touched.
		if now.Sub(swept) >= time.Second {
			for k, b := range buckets {
				if now.After(b.until) {
					delete(buckets, k)
				}
			}
			swept = now
		}
		b, known := buckets[host]
		if known && now.After(b.until) {
			b = entry{}
		}
		// A full table refuses only new clients; it never grows past the limit
		// and never locks out clients that already have a bucket.
		allowed := known || len(buckets) < limit
		if allowed {
			if b.until.IsZero() {
				b.until = now.Add(time.Minute)
			}
			b.count++
			allowed = b.count <= 240
			buckets[host] = b
		}
		mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			reply(w, r, 429, nil, "rate_limited")
			return
		}
		next.ServeHTTP(w, r)
	})
}
