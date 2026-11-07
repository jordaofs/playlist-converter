package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// Backup writes a consistent copy of the database at src to dest while the
// serving process keeps running. It deliberately bypasses Open: Open takes the
// exclusive process lock and applies the schema, and a backup must do neither.
// The copy is created with mode 0600 and never replaces an existing file.
func Backup(ctx context.Context, src, dest string) (err error) {
	if info, statErr := os.Stat(src); statErr != nil || !info.Mode().IsRegular() {
		return errors.New("backup source database not found")
	}
	if _, statErr := os.Lstat(dest); statErr == nil {
		return errors.New("backup destination already exists")
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".backup-*.db")
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", OmitHost: true, Path: src, RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}).String())
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, "VACUUM INTO ?", tmpPath); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}
	if err = verify(ctx, tmpPath); err != nil {
		return err
	}
	// A hard link publishes the finished copy atomically and, unlike rename,
	// fails instead of replacing a destination created in the meantime.
	if err = os.Link(tmpPath, dest); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}
	return nil
}

func verify(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", OmitHost: true, Path: path, RawQuery: "mode=ro"}).String())
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("verify backup: %w", err)
	}
	if result != "ok" {
		return errors.New("backup failed integrity check")
	}
	return nil
}
