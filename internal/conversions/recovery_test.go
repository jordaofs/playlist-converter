package conversions

import (
	"context"
	"errors"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"testing"
	"time"
)

func TestReviewAndSubmissionValidation(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	c := importJob(t, s)
	if _, err := s.Get(ctx, "other", c.ID); err == nil {
		t.Fatal("foreign access")
	}
	if _, err := s.Get(ctx, "p", "absent"); err == nil {
		t.Fatal("missing")
	}
	if _, err := s.Create(ctx, "p", "https://open.spotify.com/playlist/0123456789012345678901", "key"); err != nil {
		t.Fatal("replay", err)
	}
	for _, v := range []struct{ url, key string }{{"bad", "k"}, {"https://youtube.com/playlist?list=PLdifferent", "key"}, {"https://youtube.com/playlist?list=PLdifferent", "new"}, {"https://youtube.com/playlist?list=PLdifferent", ""}} {
		if _, err := s.Create(ctx, "p", v.url, v.key); err == nil {
			t.Fatal("invalid submission", v)
		}
	}
	cases := [][]domain.Decision{nil, {{ItemID: "foreign", Action: "skip"}}, {{ItemID: c.Items[0].ID, Action: "unknown"}}, {{ItemID: c.Items[0].ID, CandidateID: "other", Action: "choose"}}, {{ItemID: c.Items[0].ID, Action: "skip"}, {ItemID: c.Items[0].ID, Action: "skip"}}}
	for _, changes := range cases {
		if _, err := s.Decide(ctx, "p", c.ID, c.Revision, changes); err == nil {
			t.Fatal("invalid decision")
		}
	}
	if _, err := s.Decide(ctx, "p", c.ID, c.Revision-1, []domain.Decision{{ItemID: c.Items[0].ID, Action: "skip"}}); err == nil {
		t.Fatal("stale revision")
	}
	c, err := s.Decide(ctx, "p", c.ID, c.Revision, []domain.Decision{{ItemID: c.Items[0].ID, Action: "skip"}, {ItemID: c.Items[1].ID, Action: "skip"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, "p", c.ID, c.Revision, "", "p"); err == nil {
		t.Fatal("empty published")
	}
	if _, err = s.Resume(ctx, "p", c.ID); err == nil {
		t.Fatal("unpaused resumed")
	}
	c, err = s.Cancel(ctx, "p", c.ID)
	if err != nil || c.Status != "cancelled" {
		t.Fatal(c, err)
	}
	if _, err = s.Cancel(ctx, "p", c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resume(ctx, "p", c.ID); err == nil {
		t.Fatal("cancel resumed")
	}
	if active, e := s.HasActive(ctx, "p"); e != nil || active {
		t.Fatal(active, e)
	}
}
func TestImportRestrictionsAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Service, *fakeProvider)
		want   string
	}{
		{"owner", func(s *Service, f *fakeProvider) { s.Providers["spotify"].(*fakeProvider).identity = "foreign" }, "playlist_not_owned"},
		{"empty", func(s *Service, f *fakeProvider) { s.Providers["spotify"].(*fakeProvider).songs = nil }, "empty_playlist"},
		{"limit", func(s *Service, f *fakeProvider) { s.MaxItems = 1 }, "playlist_too_large"},
		{"identity", func(s *Service, f *fakeProvider) {
			s.Tokens = func(context.Context, string, string) (domain.Token, domain.Identity, error) {
				return domain.Token{}, domain.Identity{ID: "foreign"}, nil
			}
		}, "account_changed"},
		{"auth", func(s *Service, f *fakeProvider) {
			s.Tokens = func(context.Context, string, string) (domain.Token, domain.Identity, error) {
				return domain.Token{}, domain.Identity{}, errors.New("revoked")
			}
		}, "authorization_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := setup(t)
			ctx := context.Background()
			c, e := s.Create(ctx, "p", "https://open.spotify.com/playlist/0123456789012345678901", "k")
			if e != nil {
				t.Fatal(e)
			}
			tc.change(s, f)
			_ = s.Step(ctx, c.ID)
			c, _ = s.Get(ctx, "p", c.ID)
			if c.Error != tc.want && c.PauseReason != tc.want {
				t.Fatal(c)
			}
		})
	}
}
func TestAmbiguousIntentNeverReplayed(t *testing.T) {
	s, dst := setup(t)
	ctx := context.Background()
	c := importJob(t, s)
	c, _ = s.Publish(ctx, "p", c.ID, c.Revision, "Mix", "pub")
	if e := s.Store.Intent(ctx, c.ID, store.Operation{Kind: "create"}); e != nil {
		t.Fatal(e)
	}
	_ = s.Step(ctx, c.ID)
	c, _ = s.Get(ctx, "p", c.ID)
	if c.PauseReason != "unknown_remote_outcome" || dst.creates != 0 {
		t.Fatal(c)
	}
	_, _ = s.Resume(ctx, "p", c.ID)
	_ = s.Step(ctx, c.ID)
	if dst.creates != 0 {
		t.Fatal("replayed missing create")
	}
}
func TestRecoveryAfterPersistedAppendAndExternalEdit(t *testing.T) {
	s, dst := setup(t)
	ctx := context.Background()
	c := importJob(t, s)
	c, _ = s.Publish(ctx, "p", c.ID, c.Revision, "Mix", "pub")
	if e := s.Step(ctx, c.ID); e != nil {
		t.Fatal(e)
	}
	c, _ = s.Get(ctx, "p", c.ID)
	if e := s.Store.Intent(ctx, c.ID, store.Operation{Kind: "append", Position: 0, TargetID: "match"}); e != nil {
		t.Fatal(e)
	}
	// A restarted Service reconciles an intent left by a killed worker.
	dst.dest = []domain.Song{{ID: "match"}}
	restarted := New(s.Store, s.Providers, s.Tokens)
	if e := restarted.Step(ctx, c.ID); e != nil {
		t.Fatal(e)
	}
	c, _ = s.Get(ctx, "p", c.ID)
	if c.Published != 1 || dst.appends != 0 {
		t.Fatal(c)
	}
	dst.dest = append(dst.dest, domain.Song{ID: "foreign"})
	_ = restarted.Step(ctx, c.ID)
	c, _ = s.Get(ctx, "p", c.ID)
	if c.PauseReason != "destination_changed" || c.Status != "partial" {
		t.Fatal(c)
	}
	c.NextAttempt = time.Now().Add(time.Hour)
	s.Store.Save(ctx, c)
	if _, e := s.Resume(ctx, "p", c.ID); e == nil {
		t.Fatal("early resume")
	}
	if e := s.CancelPrincipal(ctx, "p"); e != nil {
		t.Fatal(e)
	}
	c, _ = s.Get(ctx, "p", c.ID)
	if c.Status != "cancelled" {
		t.Fatal(c)
	}
}
func TestWorkerStartAndUnavailable(t *testing.T) {
	s, _ := setup(t)
	src := s.Providers["spotify"].(*fakeProvider)
	src.songs = []domain.Song{{ID: "deleted", Unavailable: "deleted"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, e := s.Create(ctx, "p", "https://open.spotify.com/playlist/0123456789012345678901", "k")
	if e != nil {
		t.Fatal(e)
	}
	s.Start(ctx)
	s.Start(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		v, e := s.Get(ctx, "p", c.ID)
		if e != nil {
			t.Fatal(e)
		}
		if v.Status == "ready_to_publish" {
			if v.Items[0].Decision != "unavailable" {
				t.Fatal(v)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("worker did not progress")
}
func TestProviderUnavailableOrMissingConnections(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	s.Providers["youtube"] = nil
	if _, e := s.Create(ctx, "p", "https://youtube.com/playlist?list=PLabc", "key"); e == nil {
		t.Fatal("missing adapter")
	}
	s.Providers["youtube"] = &fakeProvider{}
	s.Tokens = func(context.Context, string, string) (domain.Token, domain.Identity, error) {
		return domain.Token{}, domain.Identity{}, errors.New("absent")
	}
	if _, e := s.Create(ctx, "p", "https://youtube.com/playlist?list=PLabc", "key"); e == nil {
		t.Fatal("missing credentials")
	}
}
