package main

import (
	"context"
	"fmt"
	"io"
	"playlistconverter/internal/store"
	"time"
)

// backupCommand implements `converter backup <destination>`. It only needs the
// database path, so operators can run it inside the serving container without
// OAuth credentials or the encryption key in its environment.
func backupCommand(args []string, getenv func(string) string, out io.Writer) int {
	if len(args) != 1 || args[0] == "" {
		fmt.Fprintln(out, "usage: converter backup <destination>")
		return 2
	}
	src := getenv("DATABASE_PATH")
	if src == "" {
		src = "data/converter.db"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if e := store.Backup(ctx, src, args[0]); e != nil {
		fmt.Fprintf(out, "backup failed: %v\n", e)
		return 1
	}
	fmt.Fprintf(out, "backup written: %s\n", args[0])
	return 0
}
