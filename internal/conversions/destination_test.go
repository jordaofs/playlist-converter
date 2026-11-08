package conversions

import (
	"context"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"testing"
	"time"
)

// laggingTarget answers not_found for a just-created destination, as YouTube
// did in production (2026-11-08: playlist created, 0 songs added, paused with
// not_found right after creation).
type laggingTarget struct {
	*fakeProvider
	readMisses, appendMisses int
	// hide makes the next reads omit the last appended song, as YouTube did
	// (production 2026-11-08: 3 songs in the playlist, the read showed 2).
	hide int
}

func (l *laggingTarget) ReadPlaylist(ctx context.Context, token domain.Token, id string) (domain.Playlist, error) {
	if id == "destination" && l.readMisses > 0 {
		l.readMisses--
		return domain.Playlist{}, &domain.ProviderError{Code: "not_found"}
	}
	p, err := l.fakeProvider.ReadPlaylist(ctx, token, id)
	if id == "destination" && l.hide > 0 && len(p.Songs) > 0 {
		l.hide--
		p.Songs = p.Songs[:len(p.Songs)-1]
	}
	return p, err
}

func (l *laggingTarget) Append(ctx context.Context, token domain.Token, id string, ids []string) error {
	if l.appendMisses > 0 {
		l.appendMisses--
		return &domain.ProviderError{Code: "not_found"}
	}
	return l.fakeProvider.Append(ctx, token, id, ids)
}

// publishing returns a conversion whose destination was just created.
func publishing(t *testing.T) (*Service, *laggingTarget, domain.Conversion) {
	t.Helper()
	s, dst := setup(t)
	target := &laggingTarget{fakeProvider: dst}
	s.Providers["youtube"] = target
	ctx := context.Background()
	c := importJob(t, s)
	c, err := s.Publish(ctx, "p", c.ID, c.Revision, "Mix", "pub")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Step(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Get(ctx, "p", c.ID)
	if c.DestinationID == "" {
		t.Fatalf("destination not created: %+v", c)
	}
	return s, target, c
}

// due clears NextAttempt so the next Step runs now instead of in 10 s.
func due(t *testing.T, s *Service, id string) {
	t.Helper()
	c, _ := s.Get(context.Background(), "p", id)
	c.NextAttempt = time.Time{}
	if _, err := s.save(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func finish(t *testing.T, s *Service, id string) domain.Conversion {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		due(t, s, id)
		_ = s.Step(ctx, id)
		if c, _ := s.Get(ctx, "p", id); c.Status == "completed" || c.PauseReason != "" {
			return c
		}
	}
	c, _ := s.Get(ctx, "p", id)
	return c
}

func TestDestinationCreatedAtSet(t *testing.T) {
	_, _, c := publishing(t)
	if time.Since(c.DestinationCreatedAt) > time.Minute || c.DestinationCreatedAt.IsZero() {
		t.Fatalf("created at %v", c.DestinationCreatedAt)
	}
}

func TestNewDestinationNotFoundRetries(t *testing.T) {
	s, target, c := publishing(t)
	target.readMisses = 2
	if err := s.Step(context.Background(), c.ID); err != nil {
		t.Fatalf("quiet retry reported %v", err)
	}
	c, _ = s.Get(context.Background(), "p", c.ID)
	wait := time.Until(c.NextAttempt)
	if c.PauseReason != "" || c.Status != "publishing" || wait <= 0 || wait > 15*time.Second {
		t.Fatalf("status=%q pause=%q wait=%v", c.Status, c.PauseReason, wait)
	}
	c = finish(t, s, c.ID)
	if c.Status != "completed" || c.Published != 2 || len(target.dest) != 2 {
		t.Fatalf("status=%q pause=%q published=%d dest=%d", c.Status, c.PauseReason, c.Published, len(target.dest))
	}
}

func TestNewDestinationAppendNotFoundRetries(t *testing.T) {
	s, target, c := publishing(t)
	target.appendMisses = 1
	_ = s.Step(context.Background(), c.ID)
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "" || c.NextAttempt.IsZero() {
		t.Fatalf("append 404 paused: %+v", c)
	}
	if _, err := s.Store.Operation(context.Background(), c.ID); !store.IsMissing(err) {
		t.Fatalf("definitive rejection kept the intent: %v", err)
	}
	c = finish(t, s, c.ID)
	if c.Status != "completed" || c.Published != 2 || len(target.dest) != 2 {
		t.Fatalf("status=%q pause=%q published=%d dest=%v", c.Status, c.PauseReason, c.Published, target.dest)
	}
}

func TestOldDestinationNotFoundPauses(t *testing.T) {
	s, target, c := publishing(t)
	c.DestinationCreatedAt = time.Now().Add(-10 * time.Minute)
	if _, err := s.save(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	target.readMisses = 1
	_ = s.Step(context.Background(), c.ID)
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "not_found" || c.Status != "partial" {
		t.Fatalf("status=%q pause=%q", c.Status, c.PauseReason)
	}
}

// Conversions created before this field existed keep the old behaviour.
func TestDestinationWithoutTimestampPauses(t *testing.T) {
	s, target, c := publishing(t)
	c.DestinationCreatedAt = time.Time{}
	if _, err := s.save(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	target.readMisses = 1
	_ = s.Step(context.Background(), c.ID)
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "not_found" || c.Status != "partial" {
		t.Fatalf("status=%q pause=%q", c.Status, c.PauseReason)
	}
}

// An acknowledged append that the next read does not show yet is re-read
// later, never appended again.
func TestAppendNotYetVisibleRetries(t *testing.T) {
	s, target, c := publishing(t)
	target.hide = 2
	if err := s.Step(context.Background(), c.ID); err != nil {
		t.Fatalf("quiet retry reported %v", err)
	}
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "" || c.Status != "publishing" || c.NextAttempt.IsZero() {
		t.Fatalf("status=%q pause=%q next=%v", c.Status, c.PauseReason, c.NextAttempt)
	}
	c = finish(t, s, c.ID)
	if c.Status != "completed" || c.Published != 2 || len(target.dest) != 2 || target.appends != 2 {
		t.Fatalf("status=%q pause=%q published=%d dest=%d appends=%d", c.Status, c.PauseReason, c.Published, len(target.dest), target.appends)
	}
}

// The check before the next append may also see the playlist one song behind.
func TestStaleReadBeforeNextAppendRetries(t *testing.T) {
	s, target, c := publishing(t)
	_ = s.Step(context.Background(), c.ID) // append and confirm song 1
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.Published != 1 {
		t.Fatalf("published=%d pause=%q", c.Published, c.PauseReason)
	}
	target.hide = 1
	_ = s.Step(context.Background(), c.ID)
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "" || target.appends != 1 {
		t.Fatalf("stale read paused=%q appends=%d", c.PauseReason, target.appends)
	}
	c = finish(t, s, c.ID)
	if c.Status != "completed" || len(target.dest) != 2 || target.appends != 2 {
		t.Fatalf("status=%q dest=%d appends=%d", c.Status, len(target.dest), target.appends)
	}
}

// After the grace period a write that never shows up stays ambiguous.
func TestAppendStillMissingAfterGracePauses(t *testing.T) {
	s, target, c := publishing(t)
	target.hide = 1
	_ = s.Step(context.Background(), c.ID) // append song 1; the read hides it
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "" || c.AppendedAt.IsZero() {
		t.Fatalf("first miss should retry quietly: pause=%q appendedAt=%v", c.PauseReason, c.AppendedAt)
	}
	c.AppendedAt = time.Now().Add(-10 * time.Minute)
	c.NextAttempt = time.Time{}
	if _, err := s.save(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	target.hide = 1
	_ = s.Step(context.Background(), c.ID)
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "unknown_remote_outcome" || target.appends != 1 {
		t.Fatalf("pause=%q appends=%d", c.PauseReason, target.appends)
	}
}

// A destination that differs from what was written still pauses at once.
func TestUnexpectedDestinationContentPauses(t *testing.T) {
	s, target, c := publishing(t)
	target.dest = []domain.Song{{ID: "someone-else"}}
	_ = s.Step(context.Background(), c.ID)
	c, _ = s.Get(context.Background(), "p", c.ID)
	if c.PauseReason != "destination_changed" || target.appends != 0 {
		t.Fatalf("pause=%q appends=%d", c.PauseReason, target.appends)
	}
}
