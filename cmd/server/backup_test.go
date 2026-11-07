package main

import (
	"bytes"
	"os"
	"path/filepath"
	"playlistconverter/internal/store"
	"strings"
	"testing"
)

func TestBackupCommand(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "converter.db")
	db, e := store.Open(src)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	env := map[string]string{"DATABASE_PATH": src}
	get := func(k string) string { return env[k] }
	dest := filepath.Join(dir, "copy.db")
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"missing destination", nil, 2, "usage: converter backup <destination>"},
		{"extra arguments", []string{dest, "more"}, 2, "usage: converter backup <destination>"},
		{"copies without OAuth configuration", []string{dest}, 0, "backup written"},
		{"refuses to overwrite", []string{dest}, 1, "backup failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if code := backupCommand(tc.args, get, &out); code != tc.code {
				t.Fatalf("exit %d want %d: %s", code, tc.code, out.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output %q lacks %q", out.String(), tc.want)
			}
		})
	}
	if _, e = os.Stat(dest); e != nil {
		t.Fatal("backup not created", e)
	}
}
