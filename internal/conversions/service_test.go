package conversions

import (
	"context"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"testing"
)

type fakeProvider struct {
	identity                     string
	songs                        []domain.Song
	dest                         []domain.Song
	creates, appends             int
	unknownCreate, unknownAppend bool
	created                      bool
}

func (f *fakeProvider) Identity(context.Context, domain.Token) (domain.Identity, error) {
	return domain.Identity{ID: f.identity}, nil
}
func (f *fakeProvider) ReadPlaylist(_ context.Context, _ domain.Token, id string) (domain.Playlist, error) {
	if id == "destination" {
		return domain.Playlist{ID: id, OwnerID: f.identity, Songs: append([]domain.Song(nil), f.dest...), URL: "https://youtube.com/playlist?list=PLdest"}, nil
	}
	return domain.Playlist{ID: id, Name: "My mix", OwnerID: f.identity, Songs: f.songs}, nil
}
func (f *fakeProvider) Search(_ context.Context, _ domain.Token, s domain.Song) ([]domain.Song, error) {
	return []domain.Song{{ID: "match", Title: s.Title, Artists: s.Artists, DurationMS: s.DurationMS}}, nil
}
func (f *fakeProvider) CreatePlaylist(context.Context, domain.Token, string, string) (domain.Playlist, error) {
	f.creates++
	f.created = true
	p := domain.Playlist{ID: "destination", OwnerID: f.identity, URL: "https://youtube.com/playlist?list=PLdest"}
	if f.unknownCreate {
		f.unknownCreate = false
		return domain.Playlist{}, &domain.ProviderError{Code: "network", UnknownOutcome: true}
	}
	return p, nil
}
func (f *fakeProvider) Append(_ context.Context, _ domain.Token, _ string, ids []string) error {
	f.appends++
	for _, id := range ids {
		f.dest = append(f.dest, domain.Song{ID: id})
	}
	if f.unknownAppend {
		f.unknownAppend = false
		return &domain.ProviderError{Code: "network", UnknownOutcome: true}
	}
	return nil
}
func (f *fakeProvider) FindCreated(context.Context, domain.Token, string) ([]domain.Playlist, error) {
	if f.created {
		return []domain.Playlist{{ID: "destination", OwnerID: f.identity, URL: "https://youtube.com/playlist?list=PLdest"}}, nil
	}
	return nil, nil
}
func setup(t *testing.T) (*Service, *fakeProvider) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.DB.Exec(`INSERT INTO principals VALUES('p',0,9999999999)`)
	if err != nil {
		t.Fatal(err)
	}
	song := domain.Song{ID: "original", Title: "Unique Song", Artists: []string{"Artist"}, DurationMS: 180000}
	src := &fakeProvider{identity: "spotify-owner", songs: []domain.Song{song, song}}
	dst := &fakeProvider{identity: "youtube-owner"}
	s := New(db, map[string]domain.Provider{"spotify": src, "youtube": dst}, func(_ context.Context, _ string, p string) (domain.Token, domain.Identity, error) {
		return domain.Token{}, domain.Identity{ID: p + "-owner"}, nil
	})
	return s, dst
}
func importJob(t *testing.T, s *Service) domain.Conversion {
	t.Helper()
	ctx := context.Background()
	c, err := s.Create(ctx, "p", "https://open.spotify.com/playlist/0123456789012345678901", "key")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if err = s.Step(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
	}
	c, err = s.Get(ctx, "p", c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Items) != 2 {
		t.Fatalf("snapshot missing: %+v", c)
	}
	changes := []domain.Decision{}
	for _, item := range c.Items {
		changes = append(changes, domain.Decision{ItemID: item.ID, CandidateID: "match", Action: "choose"})
	}
	c, err = s.Decide(ctx, "p", c.ID, c.Revision, changes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestConversionPreservesDuplicatesAndIdempotency(t *testing.T) {
	s, dst := setup(t)
	ctx := context.Background()
	c := importJob(t, s)
	c, err := s.Publish(ctx, "p", c.ID, c.Revision, "My mix", "publish-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, "p", c.ID, c.Revision, "My mix", "publish-key"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err = s.Step(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
	}
	c, _ = s.Get(ctx, "p", c.ID)
	if c.Status != "completed" || c.Published != 2 || dst.creates != 1 || len(dst.dest) != 2 {
		t.Fatalf("bad output %+v creates=%d dest=%v", c, dst.creates, dst.dest)
	}
}
func TestUnknownWritesReconcileWithoutDuplicates(t *testing.T) {
	s, dst := setup(t)
	ctx := context.Background()
	c := importJob(t, s)
	dst.unknownCreate = true
	dst.unknownAppend = true
	c, err := s.Publish(ctx, "p", c.ID, c.Revision, "My mix", "publish")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		_ = s.Step(ctx, c.ID)
		c, _ = s.Get(ctx, "p", c.ID)
		if c.PauseReason != "" {
			if _, err = s.Resume(ctx, "p", c.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	c, _ = s.Get(ctx, "p", c.ID)
	if c.Status != "completed" || dst.creates != 1 || dst.appends != 2 || len(dst.dest) != 2 {
		t.Fatalf("reconcile %+v create=%d append=%d", c, dst.creates, dst.appends)
	}
}
