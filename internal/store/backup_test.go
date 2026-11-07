package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestBackupWhileServing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "converter.db")
	live, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	if _, err = live.DB.Exec(`INSERT INTO principals VALUES('before',0,9999999999)`); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	var writes sync.WaitGroup
	writes.Add(1)
	go func() {
		defer writes.Done()
		for i := 0; ctx.Err() == nil; i++ {
			_, _ = live.DB.Exec(`INSERT INTO principals VALUES(?,0,9999999999)`, fmt.Sprintf("p%d", i))
		}
	}()
	dest := filepath.Join(t.TempDir(), "backup.db")
	err = Backup(context.Background(), src, dest)
	stop()
	writes.Wait()
	if err != nil {
		t.Fatalf("backup while the live store holds its lock: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode %v, want 0600", info.Mode().Perm())
	}
	restored, err := Open(dest)
	if err != nil {
		t.Fatalf("restored copy does not open: %v", err)
	}
	defer restored.Close()
	var check string
	if err = restored.DB.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil || check != "ok" {
		t.Fatalf("integrity %q %v", check, err)
	}
	var count int
	if err = restored.DB.QueryRow(`SELECT count(*) FROM principals WHERE id='before'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed row missing from backup: %d %v", count, err)
	}
}

func TestBackupRefusesUnsafeTargets(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "converter.db")
	live, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	live.Close()
	existing := filepath.Join(dir, "existing.db")
	if err = os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = Backup(context.Background(), src, existing); err == nil {
		t.Fatal("overwrote an existing file")
	}
	if b, _ := os.ReadFile(existing); string(b) != "keep" {
		t.Fatal("existing file modified")
	}
	if err = Backup(context.Background(), filepath.Join(dir, "missing.db"), filepath.Join(dir, "out.db")); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err = os.Stat(filepath.Join(dir, "missing.db")); !os.IsNotExist(err) {
		t.Fatal("backup created the missing source database")
	}
	missingDir := filepath.Join(dir, "absent", "out.db")
	if err = Backup(context.Background(), src, missingDir); err == nil {
		t.Fatal("missing destination directory accepted")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "converter.db" && e.Name() != "converter.db.lock" && e.Name() != "existing.db" && e.Name() != "converter.db-wal" && e.Name() != "converter.db-shm" {
			t.Fatalf("partial file left behind: %s", e.Name())
		}
	}
}
