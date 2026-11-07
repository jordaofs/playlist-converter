package store

import (
	"context"
	"path/filepath"
	"playlistconverter/internal/domain"
	"testing"
	"time"
)

func TestStoreTransactionsAndCleanup(t *testing.T) {
	ctx := context.Background()
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.DB.Exec(`INSERT INTO principals VALUES('p',0,9999999999)`)
	c := domain.Conversion{ID: "a", PrincipalID: "p", SourceIdentity: "s", TargetIdentity: "t", UpdatedAt: time.Now()}
	if e = s.Save(ctx, c); e != nil {
		t.Fatal(e)
	}
	if e = s.SaveKey(ctx, c, "create", "k", "v"); e != nil {
		t.Fatal(e)
	}
	p, id, e := s.Idempotency(ctx, "p", "create", "k")
	if e != nil || p != "v" || id != "a" {
		t.Fatal(p, id, e)
	}
	c.Status = "changed"
	if e = s.SaveKey(ctx, c, "create", "k", "other"); e == nil {
		t.Fatal("duplicate key")
	}
	got, _ := s.Get(ctx, "a")
	if got.Status == "changed" {
		t.Fatal("transaction not rolled back")
	}
	for _, owner := range []string{"", "p", "other"} {
		v, e := s.List(ctx, owner)
		if e != nil {
			t.Fatal(e)
		}
		if owner == "other" && len(v) != 0 {
			t.Fatal(v)
		}
	}
	if e = s.Intent(ctx, "missing", Operation{Kind: "create"}); e == nil {
		t.Fatal("missing foreign key")
	}
	if e = s.ReserveQuota(ctx, "s", "d", 0, 2); e == nil {
		t.Fatal("zero cost")
	}
	if e = s.ReserveQuota(ctx, "s", "d", 3, 2); e == nil {
		t.Fatal("too expensive")
	}
	if e = s.ReserveQuota(ctx, "s", "d", 1, 2); e != nil {
		t.Fatal(e)
	}
	if e = s.ReserveQuota(ctx, "s", "d", 1, 2); e != nil {
		t.Fatal(e)
	}
	s.DB.Exec(`INSERT INTO sessions VALUES('s','p','c',0)`)
	s.DB.Exec(`INSERT INTO oauth_attempts VALUES('o','p','s','spotify','v',0)`)
	if e = s.Cleanup(ctx, time.Now()); e != nil {
		t.Fatal(e)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM sessions").Scan(&n)
	if n != 0 {
		t.Fatal("expired session")
	}
}
func TestStoreFailures(t *testing.T) {
	if _, e := Open(""); e == nil {
		t.Fatal("empty path")
	}
	p := filepath.Join(t.TempDir(), "database")
	s, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	ctx := context.Background()
	c := domain.Conversion{ID: "a"}
	if e = s.Save(ctx, c); e == nil {
		t.Fatal("closed save")
	}
	if _, e = s.Get(ctx, "a"); e == nil {
		t.Fatal("closed get")
	}
	if _, e = s.List(ctx, ""); e == nil {
		t.Fatal("closed list")
	}
	if e = s.SaveKey(ctx, c, "s", "k", "p"); e == nil {
		t.Fatal("closed transaction")
	}
	if e = s.Cleanup(ctx, time.Now()); e == nil {
		t.Fatal("closed cleanup")
	}
	if e = s.ReserveQuota(ctx, "b", "d", 1, 2); e == nil {
		t.Fatal("closed quota")
	}
}
