package httpapi

import (
	"net/http"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"strings"
	"time"
)

// QuotaTransport reserves project-wide quota before dispatch, including reads.
// It never retries a request; an exhausted local budget means no write was sent.
type QuotaTransport struct {
	Store                  *store.Store
	Base                   http.RoundTripper
	SearchLimit, UnitLimit int
	Location               *time.Location
}

func (q *QuotaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasPrefix(r.URL.Path, "/youtube/v3/") {
		location := q.Location
		if location == nil {
			location = time.UTC
		}
		now := time.Now().In(location)
		bucket, cost, limit := "youtube:general", 1, q.UnitLimit
		if strings.HasSuffix(r.URL.Path, "/search") {
			bucket, cost, limit = "youtube:search", 1, q.SearchLimit
		} else if r.Method != "GET" {
			cost = 50
		}
		if e := q.Store.ReserveQuota(r.Context(), bucket, now.Format("2006-01-02"), cost, limit); e != nil {
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, location)
			return nil, &domain.ProviderError{Code: "quota_exhausted", RetryAfter: time.Until(next)}
		}
	}
	base := q.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}
