package conversions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"strings"
	"sync"
	"time"
)

type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string { return e.Code }

type TokenSource func(context.Context, string, string) (domain.Token, domain.Identity, error)
type Service struct {
	Store      *store.Store
	Providers  map[string]domain.Provider
	Tokens     TokenSource
	MaxItems   int
	Log        *slog.Logger
	mu         sync.Mutex
	stepMu     sync.Mutex
	stepErrors map[string]stepFailure
	start      sync.Once
}

func New(s *store.Store, p map[string]domain.Provider, t TokenSource) *Service {
	return &Service{Store: s, Providers: p, Tokens: t, MaxItems: 100}
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func fail(code string, status int) error { return &Error{Code: code, Status: status} }
func active(c domain.Conversion) bool {
	return c.Status != "completed" && c.Status != "failed" && c.Status != "cancelled"
}
func (s *Service) Get(ctx context.Context, p, id string) (domain.Conversion, error) {
	c, err := s.Store.Get(ctx, id)
	if store.IsMissing(err) || err == nil && c.PrincipalID != p {
		return domain.Conversion{}, fail("not_found", 404)
	}
	return c, err
}
func (s *Service) List(ctx context.Context, p string) ([]domain.Conversion, error) {
	return s.Store.List(ctx, p)
}
func (s *Service) save(ctx context.Context, c domain.Conversion) (domain.Conversion, error) {
	c.UpdatedAt = time.Now().UTC()
	err := s.Store.Save(ctx, c)
	return c, err
}
func (s *Service) Create(ctx context.Context, p, raw, key string) (domain.Conversion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var c domain.Conversion
	provider, id, err := ParseSource(raw)
	if err != nil {
		return c, err
	}
	if key == "" || len(key) > 128 {
		return c, fail("invalid_idempotency_key", 400)
	}
	payload := provider + ":" + id
	old, jid, err := s.Store.Idempotency(ctx, p, "create", key)
	if err == nil {
		if old != payload {
			return c, fail("idempotency_conflict", 409)
		}
		return s.Get(ctx, p, jid)
	}
	if !store.IsMissing(err) {
		return c, err
	}
	list, err := s.List(ctx, p)
	if err != nil {
		return c, err
	}
	for _, v := range list {
		if active(v) {
			return c, fail("active_conversion_exists", 409)
		}
	}
	target := "spotify"
	if provider == "spotify" {
		target = "youtube"
	}
	if s.Providers[provider] == nil || s.Providers[target] == nil {
		return c, fail("provider_unavailable", 503)
	}
	_, src, err := s.Tokens(ctx, p, provider)
	if err != nil {
		return c, fail("connection_required", 401)
	}
	_, dst, err := s.Tokens(ctx, p, target)
	if err != nil {
		return c, fail("connection_required", 401)
	}
	now := time.Now().UTC()
	c = domain.Conversion{ID: newID(), PrincipalID: p, SourceProvider: provider, TargetProvider: target, SourceID: id, SourceIdentity: src.ID, TargetIdentity: dst.ID, Status: "queued", Revision: 1, Items: []domain.Item{}, CreatedAt: now, UpdatedAt: now}
	if err = s.Store.SaveKey(ctx, c, "create", key, payload); err != nil {
		return c, err
	}
	s.event("conversion.started", c, "")
	return c, nil
}
func (s *Service) Decide(ctx context.Context, p, id string, revision int, changes []domain.Decision) (domain.Conversion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.Get(ctx, p, id)
	if err != nil {
		return c, err
	}
	if c.Revision != revision || (c.Status != "needs_review" && c.Status != "ready_to_publish") || c.PauseReason != "" {
		return c, fail("revision_conflict", 409)
	}
	if len(changes) == 0 || len(changes) > s.MaxItems {
		return c, fail("invalid_decisions", 400)
	}
	items := append([]domain.Item(nil), c.Items...)
	seen := map[string]bool{}
	for _, change := range changes {
		if seen[change.ItemID] {
			return c, fail("invalid_decisions", 400)
		}
		seen[change.ItemID] = true
		found := false
		for i, item := range items {
			if item.ID != change.ItemID {
				continue
			}
			found = true
			if change.Action == "skip" {
				item.Decision = "skipped"
				item.SelectedID = ""
				item.Reason = "Skipped by you"
			} else if change.Action == "choose" {
				valid := false
				for _, candidate := range item.Candidates {
					if candidate.Song.ID == change.CandidateID && candidate.Song.Unavailable == "" {
						valid = true
					}
				}
				if !valid {
					return c, fail("invalid_candidate", 400)
				}
				item.Decision = "manual"
				item.SelectedID = change.CandidateID
				item.Reason = "Selected by you"
			} else {
				return c, fail("invalid_decisions", 400)
			}
			items[i] = item
		}
		if !found {
			return c, fail("invalid_item", 400)
		}
	}
	c.Items = items
	c.Revision++
	c.Status = reviewState(items)
	return s.save(ctx, c)
}
func reviewState(items []domain.Item) string {
	for _, i := range items {
		if i.Decision == "review" || i.Decision == "pending" {
			return "needs_review"
		}
	}
	return "ready_to_publish"
}
func selected(c domain.Conversion) []string {
	ids := []string{}
	for _, i := range c.Items {
		if i.Decision == "automatic" || i.Decision == "manual" {
			ids = append(ids, i.SelectedID)
		}
	}
	return ids
}
func (s *Service) Publish(ctx context.Context, p, id string, revision int, name, key string) (domain.Conversion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.Get(ctx, p, id)
	if err != nil {
		return c, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = c.SourceName + " · " + c.TargetProvider
	}
	if len([]rune(name)) > 100 || key == "" || len(key) > 128 {
		return c, fail("invalid_publish_request", 400)
	}
	b, _ := json.Marshal([]any{revision, name})
	payload := string(b)
	old, jid, err := s.Store.Idempotency(ctx, p, "publish:"+id, key)
	if err == nil {
		if old != payload || jid != id {
			return c, fail("idempotency_conflict", 409)
		}
		return c, nil
	}
	if !store.IsMissing(err) {
		return c, err
	}
	if c.Revision != revision || c.Status != "ready_to_publish" || c.PauseReason != "" {
		return c, fail("revision_conflict", 409)
	}
	if len(selected(c)) == 0 {
		return c, fail("empty_selection", 400)
	}
	c.Name = name
	c.Status = "publishing"
	c.UpdatedAt = time.Now().UTC()
	return c, s.Store.SaveKey(ctx, c, "publish:"+id, key, payload)
}
func (s *Service) Resume(ctx context.Context, p, id string) (domain.Conversion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.Get(ctx, p, id)
	if err != nil {
		return c, err
	}
	if c.PauseReason == "" || !active(c) {
		return c, fail("invalid_state", 409)
	}
	if time.Now().Before(c.NextAttempt) {
		return c, fail("retry_later", 409)
	}
	c.PauseReason = ""
	c.Error = ""
	if c.Status == "partial" {
		c.Status = "publishing"
	}
	return s.save(ctx, c)
}
func (s *Service) Cancel(ctx context.Context, p, id string) (domain.Conversion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.Get(ctx, p, id)
	if err != nil {
		return c, err
	}
	if !active(c) {
		return c, nil
	}
	c.Status = "cancelled"
	c.PauseReason = ""
	return s.save(ctx, c)
}
func (s *Service) CancelPrincipal(ctx context.Context, p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.List(ctx, p)
	if err != nil {
		return err
	}
	for _, c := range jobs {
		if active(c) {
			c.Status = "cancelled"
			c.PauseReason = ""
			if _, err = s.save(ctx, c); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Service) HasActive(ctx context.Context, p string) (bool, error) {
	jobs, err := s.List(ctx, p)
	if err != nil {
		return false, err
	}
	for _, c := range jobs {
		if active(c) {
			return true, nil
		}
	}
	return false, nil
}
func (s *Service) credentials(ctx context.Context, c domain.Conversion, provider string) (domain.Token, error) {
	t, i, err := s.Tokens(ctx, c.PrincipalID, provider)
	if err != nil {
		return t, &domain.ProviderError{Code: "authorization_required"}
	}
	expected := c.SourceIdentity
	if provider == c.TargetProvider {
		expected = c.TargetIdentity
	}
	if i.ID != expected {
		return t, &domain.ProviderError{Code: "account_changed"}
	}
	return t, nil
}
func (s *Service) handle(ctx context.Context, c domain.Conversion, err error) error {
	var pe *domain.ProviderError
	if errors.As(err, &pe) {
		c.PauseReason = pe.Code
		if pe.UnknownOutcome {
			c.PauseReason = "unknown_remote_outcome"
		}
		if pe.RetryAfter > 0 {
			c.NextAttempt = time.Now().Add(pe.RetryAfter)
		}
		if c.DestinationID != "" {
			c.Status = "partial"
		}
	} else {
		c.Status = "failed"
		c.Error = "conversion_failed"
	}
	_, saveErr := s.save(ctx, c)
	if saveErr != nil {
		return fmt.Errorf("save failure: %w", saveErr)
	}
	if c.Status == "failed" {
		s.event("conversion.failed", c, c.Error)
	} else {
		s.event("conversion.paused", c, c.PauseReason)
	}
	return err
}
