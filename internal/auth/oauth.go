package auth

import (
	"context"
	"database/sql"
	"errors"
	"golang.org/x/oauth2"
	"net/http"
	"playlistconverter/internal/domain"
	"time"
)

// CallbackError reports why an OAuth callback failed. Code is the step that
// failed; Cause is the provider's own error code, when it gave one. Both are
// short identifiers, never tokens or account data; callers still sanitize
// Cause before logging because it comes from the provider.
type CallbackError struct{ Code, Cause string }

func (e *CallbackError) Error() string { return e.Code }

func (m *Manager) Begin(ctx context.Context, s Session, provider string) (string, error) {
	cfg := m.OAuth[provider]
	if cfg == nil || cfg.ClientID == "" {
		return "", errors.New("provider_not_configured")
	}
	state, verifier := Random(), oauth2.GenerateVerifier()
	_, e := m.DB.ExecContext(ctx, "INSERT INTO oauth_attempts(state,principal_id,session_id,provider,verifier,expires_at) VALUES(?,?,?,?,?,?)", hash(state), s.PrincipalID, s.ID, provider, verifier, time.Now().Add(10*time.Minute).Unix())
	if e != nil {
		return "", e
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier)), nil
}

// Finish completes a callback for a known session and sets the new session
// cookie. Each state works once; see Callback for duplicate deliveries.
func (m *Manager) Finish(ctx context.Context, w http.ResponseWriter, s Session, provider, state, code string) error {
	raw, e := m.complete(ctx, s, provider, state, code)
	if e != nil {
		return e
	}
	m.cookie(w, raw, int(lifetime.Seconds()))
	return nil
}

// complete runs one OAuth callback: it consumes the state, exchanges the
// code, saves the connection and rotates the session. It returns the new raw
// session token; callers set the cookie.
func (m *Manager) complete(ctx context.Context, s Session, provider, state, code string) (string, error) {
	cfg := m.OAuth[provider]
	adapter := m.Providers[provider]
	if cfg == nil || adapter == nil || state == "" || len(state) > 256 || code == "" || len(code) > 4096 {
		return "", &CallbackError{Code: "invalid_oauth_callback"}
	}
	var verifier string
	e := m.DB.QueryRowContext(ctx, "DELETE FROM oauth_attempts WHERE state=? AND principal_id=? AND session_id=? AND provider=? AND expires_at>? RETURNING verifier", hash(state), s.PrincipalID, s.ID, provider, time.Now().Unix()).Scan(&verifier)
	if e != nil {
		return "", &CallbackError{Code: "invalid_oauth_state"}
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, m.Client)
	token, e := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if e != nil {
		cause := "network"
		var re *oauth2.RetrieveError
		if errors.As(e, &re) {
			cause = re.ErrorCode
		}
		return "", &CallbackError{Code: "oauth_exchange_failed", Cause: cause}
	}
	next := domain.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, Expiry: token.Expiry}
	id, e := adapter.Identity(ctx, next)
	if e != nil {
		cause := "other"
		var pe *domain.ProviderError
		if errors.As(e, &pe) {
			cause = pe.Code
		}
		return "", &CallbackError{Code: "provider_identity_failed", Cause: cause}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var old string
	var oldCipher []byte
	e = m.DB.QueryRowContext(ctx, "SELECT identity_id,token FROM connections WHERE principal_id=? AND provider=?", s.PrincipalID, provider).Scan(&old, &oldCipher)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return "", e
	}
	if old != "" && old != id.ID && m.Active != nil && m.Active(ctx, s.PrincipalID) {
		return "", &CallbackError{Code: "account_change_requires_new_conversion"}
	}
	// Providers may omit a refresh token on repeated consent; retain it only for the same identity.
	if old == id.ID && next.RefreshToken == "" {
		if previous, err := m.decode(s.PrincipalID, provider, oldCipher); err == nil {
			next.RefreshToken = previous.RefreshToken
		}
	}
	if e = m.Save(ctx, s.PrincipalID, provider, id, next); e != nil {
		return "", e
	}
	raw := Random()
	_, e = m.DB.ExecContext(ctx, "UPDATE sessions SET id=?,csrf=?,expires_at=? WHERE id=? AND principal_id=?", hash(raw), Random(), time.Now().Add(lifetime).Unix(), s.ID, s.PrincipalID)
	if e != nil {
		return "", e
	}
	return raw, nil
}
