package main

import (
	"context"
	"playlistconverter/internal/conversions"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"testing"
	"time"
)

func TestExpireRemovesExpiredPrivateData(t *testing.T) {
	db, e := store.Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	now := time.Now()
	for _, p := range []struct {
		id     string
		expiry time.Time
	}{{"expired", now.Add(-time.Hour)}, {"current", now.Add(time.Hour)}} {
		if _, e = db.DB.Exec("INSERT INTO principals(id,created_at,expires_at)VALUES(?,?,?)", p.id, now.Unix(), p.expiry.Unix()); e != nil {
			t.Fatal(e)
		}
	}
	for _, c := range []domain.Conversion{{ID: "expired-job", PrincipalID: "expired", Status: "queued", CreatedAt: now, UpdatedAt: now}, {ID: "old-job", PrincipalID: "current", Status: "completed", CreatedAt: now.Add(-8 * 24 * time.Hour), UpdatedAt: now}, {ID: "current-job", PrincipalID: "current", Status: "completed", CreatedAt: now, UpdatedAt: now}} {
		if e = db.Save(ctx, c); e != nil {
			t.Fatal(e)
		}
	}
	engine := conversions.New(db, nil, nil)
	if e = expire(ctx, db, engine); e != nil {
		t.Fatal(e)
	}
	var n int
	db.DB.QueryRow("SELECT count(*) FROM principals").Scan(&n)
	if n != 1 {
		t.Fatal("principal retention", n)
	}
	jobs, e := engine.List(ctx, "")
	if e != nil || len(jobs) != 1 || jobs[0].ID != "current-job" {
		t.Fatal("job retention", jobs, e)
	}
	db.Close()
	if e = expire(ctx, db, engine); e == nil {
		t.Fatal("closed database ignored")
	}
}
func TestRunRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")
	if e := run(); e == nil {
		t.Fatal("unsafe configuration accepted")
	}
}
func TestCleanupCancellation(t *testing.T) {
	db, e := store.Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { cleanup(ctx, db, conversions.New(db, nil, nil)); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not stop")
	}
}
