package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed migrations/001_schema.sql
var schema string

type Store struct {
	DB   *sql.DB
	lock *os.File
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path required")
	}
	var lock *os.File
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		var err error
		lock, err = os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			lock.Close()
			return nil, errors.New("database already in use by another process")
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		if lock != nil {
			lock.Close()
		}
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, lock: lock}
	if _, err = db.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;"); err != nil {
		s.Close()
		return nil, err
	}
	if _, err = db.Exec(schema); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error {
	err := s.DB.Close()
	if s.lock != nil {
		_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		_ = s.lock.Close()
	}
	return err
}
func (s *Store) ReserveQuota(ctx context.Context, bucket, day string, cost, limit int) error {
	if cost < 1 || limit < 1 || cost > limit {
		return errors.New("quota_exhausted")
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO quotas(bucket,day,used) VALUES(?,?,?) ON CONFLICT(bucket,day) DO UPDATE SET used=used+excluded.used WHERE used+excluded.used<=?`, bucket, day, cost, limit)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("quota_exhausted")
	}
	return nil
}

// Cleanup removes expired principals and their cascaded private data. The caller
// must first stop work for expired principals through the conversion service.
func (s *Store) Cleanup(ctx context.Context, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM oauth_attempts WHERE expires_at<=?;`, now.Unix())
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=?`, now.Unix())
	return err
}
