// Package providers contains bounded, credential-safe HTTP transport shared by adapters.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"playlistconverter/internal/domain"
)

const MaxItems = 1000
const maxResponseBytes = 8 << 20

type API struct {
	Client  *http.Client
	BaseURL string
}

func NewAPI(client *http.Client, baseURL string) *API {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if clone.Timeout == 0 {
		clone.Timeout = 20 * time.Second
	}
	return &API{Client: &clone, BaseURL: strings.TrimRight(baseURL, "/")}
}

// Do never retries: mutations with ambiguous outcomes must be reconciled by the caller.
func (a *API) Do(ctx context.Context, token domain.Token, method, path string, body, out any) error {
	base, err := url.Parse(a.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return Error("invalid_provider_configuration")
	}
	if token.AccessToken == "" {
		return Error("authorization_required")
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return Error("invalid_provider_request")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, a.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return Error("invalid_provider_request")
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	mutation := method != http.MethodGet && method != http.MethodHead
	res, err := a.Client.Do(req)
	if err != nil {
		var local *domain.ProviderError
		if errors.As(err, &local) {
			return &domain.ProviderError{Code: local.Code, RetryAfter: local.RetryAfter, UnknownOutcome: local.UnknownOutcome}
		}
		return &domain.ProviderError{Code: "provider_unavailable", UnknownOutcome: mutation}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return &domain.ProviderError{Code: "invalid_provider_response", UnknownOutcome: mutation}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return responseError(res.StatusCode, res.Header.Get("Retry-After"), data, mutation)
	}
	if out != nil && json.Unmarshal(data, out) != nil {
		return &domain.ProviderError{Code: "invalid_provider_response", UnknownOutcome: mutation}
	}
	return nil
}

func responseError(status int, retry string, data []byte, mutation bool) error {
	code := "provider_rejected"
	switch {
	case status == 401:
		code = reasonCode(data, "authorization_required")
	case status == 429:
		code = "quota_exhausted"
	case status == 403:
		code = reasonCode(data, "access_denied")
	case status == 404:
		code = "not_found"
	case status >= 500:
		code = "provider_unavailable"
	}
	var wait time.Duration
	if seconds, err := strconv.Atoi(retry); err == nil && seconds > 0 {
		wait = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(retry); err == nil {
		wait = time.Until(date)
		if wait < 0 {
			wait = 0
		}
	}
	return &domain.ProviderError{Code: code, RetryAfter: wait, UnknownOutcome: mutation && status >= 500}
}

// reasonCode maps Google API error reasons to closed codes; other providers
// and unknown reasons keep the status-based fallback.
func reasonCode(data []byte, fallback string) string {
	var body struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &body) != nil {
		return fallback
	}
	for _, e := range body.Error.Errors {
		switch e.Reason {
		case "quotaExceeded", "dailyLimitExceeded", "rateLimitExceeded", "userRateLimitExceeded":
			return "quota_exhausted"
		case "accessNotConfigured":
			return "api_not_enabled"
		case "youtubeSignupRequired":
			return "channel_selection_required"
		}
	}
	return fallback
}
func Error(code string) error { return &domain.ProviderError{Code: code} }
