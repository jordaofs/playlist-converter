package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"playlistconverter/internal/domain"
)

const lifetime = 7 * 24 * time.Hour

var ErrSession = errors.New("session_required")
var ErrConnection = errors.New("connection_required")

type Session struct{ ID, PrincipalID, CSRF string }
type Connection struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Name     string `json:"name"`
}
type Manager struct {
	DB        *sql.DB
	Key       []byte
	KeyID     string
	BaseURL   string
	Secure    bool
	OAuth     map[string]*oauth2.Config
	Providers map[string]domain.Provider
	Client    *http.Client
	mu        sync.Mutex
	Active    func(context.Context, string) bool
}

func Random() string {
	v := make([]byte, 32)
	if _, e := rand.Read(v); e != nil {
		panic("system random unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(v)
}
func hash(v string) string { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }
func (m *Manager) cookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: "converter_session", Value: value, Path: "/", MaxAge: age, HttpOnly: true, Secure: m.Secure, SameSite: http.SameSiteLaxMode})
}
func (m *Manager) Session(ctx context.Context, w http.ResponseWriter, r *http.Request, create bool) (Session, error) {
	c, e := r.Cookie("converter_session")
	if e == nil {
		s := Session{ID: hash(c.Value)}
		var expiry int64
		e = m.DB.QueryRowContext(ctx, "SELECT principal_id,csrf,expires_at FROM sessions WHERE id=?", s.ID).Scan(&s.PrincipalID, &s.CSRF, &expiry)
		if e == nil && expiry > time.Now().Unix() {
			until := time.Now().Add(lifetime).Unix()
			_, e = m.DB.ExecContext(ctx, "UPDATE sessions SET expires_at=? WHERE id=?", until, s.ID)
			if e != nil {
				return Session{}, e
			}
			_, e = m.DB.ExecContext(ctx, "UPDATE principals SET expires_at=? WHERE id=?", until, s.PrincipalID)
			if e != nil {
				return Session{}, e
			}
			m.cookie(w, c.Value, int(lifetime.Seconds()))
			return s, nil
		}
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return Session{}, e
		}
	}
	if !create {
		return Session{}, ErrSession
	}
	raw := Random()
	s := Session{ID: hash(raw), PrincipalID: Random(), CSRF: Random()}
	tx, e := m.DB.BeginTx(ctx, nil)
	if e != nil {
		return Session{}, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, "INSERT INTO principals(id,created_at,expires_at) VALUES(?,?,?)", s.PrincipalID, time.Now().Unix(), time.Now().Add(lifetime).Unix())
	if e != nil {
		return Session{}, e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO sessions(id,principal_id,csrf,expires_at) VALUES(?,?,?,?)", s.ID, s.PrincipalID, s.CSRF, time.Now().Add(lifetime).Unix())
	if e != nil {
		return Session{}, e
	}
	if e = tx.Commit(); e != nil {
		return Session{}, e
	}
	m.cookie(w, raw, int(lifetime.Seconds()))
	return s, nil
}
func (m *Manager) Connections(ctx context.Context, p string) ([]Connection, error) {
	rows, e := m.DB.QueryContext(ctx, "SELECT provider,identity_id,name FROM connections WHERE principal_id=? ORDER BY provider", p)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	v := []Connection{}
	for rows.Next() {
		var c Connection
		if e = rows.Scan(&c.Provider, &c.ID, &c.Name); e != nil {
			return nil, e
		}
		v = append(v, c)
	}
	return v, rows.Err()
}
func (m *Manager) Save(ctx context.Context, p, provider string, id domain.Identity, token domain.Token) error {
	if id.ID == "" {
		return ErrConnection
	}
	data, e := json.Marshal(token)
	if e != nil {
		return e
	}
	ciphertext, e := seal(m.Key, p+"/"+provider+"/"+m.KeyID, data)
	if e != nil {
		return e
	}
	_, e = m.DB.ExecContext(ctx, "INSERT INTO connections(principal_id,provider,identity_id,name,token) VALUES(?,?,?,?,?) ON CONFLICT(principal_id,provider) DO UPDATE SET identity_id=excluded.identity_id,name=excluded.name,token=excluded.token", p, provider, id.ID, id.Name, ciphertext)
	return e
}
func (m *Manager) Token(ctx context.Context, p, provider string) (domain.Token, domain.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var id domain.Identity
	var blob []byte
	var token domain.Token
	e := m.DB.QueryRowContext(ctx, "SELECT identity_id,name,token FROM connections WHERE principal_id=? AND provider=?", p, provider).Scan(&id.ID, &id.Name, &blob)
	if errors.Is(e, sql.ErrNoRows) {
		return token, id, ErrConnection
	}
	if e != nil {
		return token, id, e
	}
	data, e := open(m.Key, p+"/"+provider+"/"+m.KeyID, blob)
	if e != nil {
		return token, id, ErrConnection
	}
	if e = json.Unmarshal(data, &token); e != nil {
		return token, id, ErrConnection
	}
	if token.Expiry.After(time.Now().Add(time.Minute)) {
		return token, id, nil
	}
	cfg := m.OAuth[provider]
	if cfg == nil || token.RefreshToken == "" {
		return domain.Token{}, id, ErrConnection
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, m.Client)
	refreshed, e := cfg.TokenSource(ctx, &oauth2.Token{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, Expiry: time.Now().Add(-time.Minute)}).Token()
	if e != nil {
		return domain.Token{}, id, ErrConnection
	}
	next := domain.Token{AccessToken: refreshed.AccessToken, RefreshToken: refreshed.RefreshToken, Expiry: refreshed.Expiry}
	if next.RefreshToken == "" {
		next.RefreshToken = token.RefreshToken
	}
	if e = m.Save(ctx, p, provider, id, next); e != nil {
		return domain.Token{}, id, e
	}
	return next, id, nil
}
func (m *Manager) decode(p, provider string, data []byte) (domain.Token, error) {
	var v domain.Token
	plain, e := open(m.Key, p+"/"+provider+"/"+m.KeyID, data)
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(plain, &v)
	return v, e
}
