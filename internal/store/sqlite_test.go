package store

import (
	"context"
	"path/filepath"
	"playlistconverter/internal/domain"
	"testing"
	"time"
)

func TestPersistenceIsolationAndQuota(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("second writer allowed")
	}
	if _, err = s.DB.Exec(`INSERT INTO principals VALUES('p',0,9999999999)`); err != nil {
		t.Fatal(err)
	}
	c := domain.Conversion{ID: "j", PrincipalID: "p", SourceIdentity: "source", TargetIdentity: "target", Status: "queued", CreatedAt: time.Now()}
	if err = s.SaveKey(ctx, c, "create", "key", "payload"); err != nil {
		t.Fatal(err)
	}
	if err = s.Intent(ctx, "j", Operation{Kind: "create"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveQuota(ctx, "search", "today", 2, 3); err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveQuota(ctx, "search", "today", 2, 3); err == nil {
		t.Fatal("quota overrun")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Get(ctx, "j")
	if err != nil || got.SourceIdentity != "source" || got.PrincipalID != "p" {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	if _, err = s.Operation(ctx, "j"); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearIntent(ctx, "j"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Operation(ctx, "j"); !IsMissing(err) {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`DELETE FROM principals WHERE id='p'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, "j"); !IsMissing(err) {
		t.Fatal("foreign key cascade missing")
	}
}
