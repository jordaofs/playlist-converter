package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"playlistconverter/internal/domain"
	"testing"

	"golang.org/x/oauth2"
)

type failingIdentity struct{ domain.Provider }

func (failingIdentity) Identity(context.Context, domain.Token) (domain.Identity, error) {
	return domain.Identity{}, &domain.ProviderError{Code: "channel_selection_required"}
}

// A failed callback must say why, as closed codes, so the operator can tell a
// wrong client secret from a missing YouTube channel without logging secrets.
func TestFinishReportsCause(t *testing.T) {
	for _, tc := range []struct {
		name, tokenStatus, tokenBody string
		provider                     domain.Provider
		code, cause                  string
	}{
		{"bad client", "401", `{"error":"invalid_client","error_description":"secret do-not-leak"}`, identityProvider{}, "oauth_exchange_failed", "invalid_client"},
		{"no channel", "200", `{"access_token":"a","token_type":"Bearer","expires_in":3600}`, failingIdentity{}, "provider_identity_failed", "channel_selection_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := manager(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.tokenStatus != "200" {
					w.WriteHeader(http.StatusUnauthorized)
				}
				io.WriteString(w, tc.tokenBody)
			}))
			defer ts.Close()
			m.OAuth["youtube"] = &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: ts.URL + "/auth", TokenURL: ts.URL + "/token"}}
			m.Providers["youtube"] = tc.provider
			s, _ := session(t, m)
			link, e := m.Begin(context.Background(), s, "youtube")
			if e != nil {
				t.Fatal(e)
			}
			u, _ := url.Parse(link)
			e = m.Finish(context.Background(), httptest.NewRecorder(), s, "youtube", u.Query().Get("state"), "code")
			var ce *CallbackError
			if !errors.As(e, &ce) || ce.Code != tc.code || ce.Cause != tc.cause {
				t.Fatalf("got %#v", e)
			}
			if e.Error() != tc.code {
				t.Fatalf("message %q exposes more than the code", e.Error())
			}
		})
	}
}

func TestFinishInvalidStateHasNoCause(t *testing.T) {
	m := manager(t)
	s, _ := session(t, m)
	m.OAuth["youtube"] = &oauth2.Config{ClientID: "test"}
	m.Providers["youtube"] = identityProvider{}
	e := m.Finish(context.Background(), httptest.NewRecorder(), s, "youtube", "unknown", "code")
	var ce *CallbackError
	if !errors.As(e, &ce) || ce.Code != "invalid_oauth_state" || ce.Cause != "" {
		t.Fatalf("got %#v", e)
	}
}
