package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"playlistconverter/internal/domain"
	"testing"
	"time"
)

func TestTransport(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, code string
		unknown    bool
	}{
		{401, `{"secret":"do-not-leak"}`, "authorization_required", false},
		{403, `{"error":{"errors":[{"reason":"quotaExceeded"}]}}`, "quota_exhausted", false},
		{403, `{"error":{"errors":[{"reason":"accessNotConfigured"}]}}`, "api_not_enabled", false},
		{401, `{"error":{"errors":[{"reason":"youtubeSignupRequired"}]}}`, "channel_selection_required", false},
		{403, `{"error":{"errors":[{"reason":"youtubeSignupRequired"}]}}`, "channel_selection_required", false},
		{403, `{}`, "access_denied", false}, {404, `{}`, "not_found", false},
		{429, `{}`, "quota_exhausted", false}, {500, `{}`, "provider_unavailable", true},
		{400, `{}`, "provider_rejected", false},
	} {
		t.Run(tc.code+http.StatusText(tc.status), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "3")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			err := NewAPI(s.Client(), s.URL).Do(context.Background(), domain.Token{AccessToken: "token"}, http.MethodPost, "/thing", map[string]string{}, nil)
			var pe *domain.ProviderError
			if !errors.As(err, &pe) || pe.Code != tc.code || pe.UnknownOutcome != tc.unknown || pe.RetryAfter != 3*time.Second {
				t.Fatalf("unexpected error: %#v", err)
			}
		})
	}
}
func TestTransportSuccessAndMalformed(t *testing.T) {
	for _, body := range []string{`{"value":42}`, `not-json`} {
		t.Run(body, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Error("missing token")
				}
				_, _ = w.Write([]byte(body))
			}))
			defer s.Close()
			var out struct{ Value int }
			err := NewAPI(s.Client(), s.URL).Do(context.Background(), domain.Token{AccessToken: "token"}, "GET", "/thing", nil, &out)
			if body[0] == '{' {
				if err != nil || out.Value != 42 {
					t.Fatalf("%v %+v", err, out)
				}
			} else if err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
}
func TestTransportNoRedirect(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	err := NewAPI(origin.Client(), origin.URL).Do(context.Background(), domain.Token{AccessToken: "token"}, "GET", "/", nil, nil)
	if err == nil || called {
		t.Fatal("redirect was followed or accepted")
	}
}

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }
func TestLocalQuotaErrorPreserved(t *testing.T) {
	original := &domain.ProviderError{Code: "quota_exhausted", RetryAfter: time.Hour}
	api := NewAPI(&http.Client{Transport: failingTransport{original}}, "https://api.example.com")
	err := api.Do(context.Background(), domain.Token{AccessToken: "token"}, "POST", "/items", nil, nil)
	var got *domain.ProviderError
	if !errors.As(err, &got) || got.Code != original.Code || got.UnknownOutcome || got.RetryAfter != time.Hour {
		t.Fatalf("lost preflight error: %#v", err)
	}
}
func TestTransportInputAndNetworkFailures(t *testing.T) {
	for _, tc := range []struct{ base, path, token string }{{"not a URL", "/", "x"}, {"https://a.test", "//evil", "x"}, {"https://a.test", "/", ""}} {
		if err := NewAPI(nil, tc.base).Do(context.Background(), domain.Token{AccessToken: tc.token}, "GET", tc.path, nil, nil); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	api := NewAPI(&http.Client{Transport: failingTransport{errors.New("secret transport details")}}, "https://api.example.com")
	err := api.Do(context.Background(), domain.Token{AccessToken: "token"}, "POST", "/items", nil, nil)
	var pe *domain.ProviderError
	if !errors.As(err, &pe) || !pe.UnknownOutcome || pe.Code != "provider_unavailable" {
		t.Fatal(err)
	}
	if err := api.Do(context.Background(), domain.Token{AccessToken: "token"}, "POST", "/items", make(chan int), nil); err == nil {
		t.Fatal("unserializable body accepted")
	}
}
