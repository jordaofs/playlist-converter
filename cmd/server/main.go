package main

import (
	"context"
	"errors"
	"golang.org/x/oauth2"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"playlistconverter/internal/auth"
	"playlistconverter/internal/config"
	"playlistconverter/internal/conversions"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/httpapi"
	"playlistconverter/internal/providers/spotify"
	"playlistconverter/internal/providers/youtube"
	"playlistconverter/internal/store"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if len(os.Args) > 1 && os.Args[1] == "backup" {
		os.Exit(backupCommand(os.Args[2:], os.Getenv, os.Stdout))
	}
	if e := run(); e != nil {
		slog.Error("server stopped", "error", e)
		os.Exit(1)
	}
}
func run() error {
	cfg, e := config.Load(os.Getenv)
	if e != nil {
		return e
	}
	db, e := store.Open(cfg.DatabasePath)
	if e != nil {
		return e
	}
	defer db.Close()
	location, e := time.LoadLocation("America/Los_Angeles")
	if e != nil {
		return e
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }, Transport: &httpapi.QuotaTransport{Store: db, SearchLimit: 100, UnitLimit: 10000, Location: location}}
	providers := map[string]domain.Provider{"spotify": spotify.New(client, ""), "youtube": youtube.New(client, "")}
	manager := &auth.Manager{DB: db.DB, Key: cfg.Key, KeyID: cfg.KeyID, BaseURL: cfg.BaseURL, Secure: strings.HasPrefix(cfg.BaseURL, "https://"), Client: client, Providers: providers, OAuth: map[string]*oauth2.Config{
		"spotify": {ClientID: cfg.SpotifyID, ClientSecret: cfg.SpotifySecret, RedirectURL: cfg.BaseURL + "/api/auth/spotify/callback", Scopes: []string{"playlist-read-private", "playlist-modify-private"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.spotify.com/authorize", TokenURL: "https://accounts.spotify.com/api/token"}},
		"youtube": {ClientID: cfg.YouTubeID, ClientSecret: cfg.YouTubeSecret, RedirectURL: cfg.BaseURL + "/api/auth/youtube/callback", Scopes: []string{"https://www.googleapis.com/auth/youtube.force-ssl"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}},
	}}
	engine := conversions.New(db, providers, manager.Token)
	manager.Active = func(ctx context.Context, p string) bool {
		active, e := engine.HasActive(ctx, p)
		return e != nil || active
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	engine.Start(ctx)
	go cleanup(ctx, db, engine)
	handler := (&httpapi.Server{Config: cfg, Auth: manager, Engine: engine}).Handler()
	server := &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("playlist converter listening", "address", cfg.Address)
	e = server.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
func cleanup(ctx context.Context, db *store.Store, engine *conversions.Service) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if e := expire(ctx, db, engine); e != nil {
				slog.Error("retention cleanup failed", "error", e)
			}
		}
	}
}
func expire(ctx context.Context, db *store.Store, engine *conversions.Service) error {
	rows, e := db.DB.QueryContext(ctx, "SELECT id FROM principals WHERE expires_at<=?", time.Now().Unix())
	if e != nil {
		return e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = engine.CancelPrincipal(ctx, id); e != nil {
			return e
		}
		if _, e = db.DB.ExecContext(ctx, "DELETE FROM principals WHERE id=? AND expires_at<=?", id, time.Now().Unix()); e != nil {
			return e
		}
	}
	// Terminated jobs expire even when the browser keeps its session active.
	jobs, e := engine.List(ctx, "")
	if e != nil {
		return e
	}
	for _, job := range jobs {
		if time.Since(job.CreatedAt) > 7*24*time.Hour {
			if _, e = engine.Cancel(ctx, job.PrincipalID, job.ID); e != nil {
				return e
			}
			if _, e = db.DB.ExecContext(ctx, "DELETE FROM conversions WHERE id=?", job.ID); e != nil {
				return e
			}
		}
	}
	if _, e = db.DB.ExecContext(ctx, "DELETE FROM quotas WHERE day<?", time.Now().Add(-8*24*time.Hour).Format("2006-01-02")); e != nil {
		return e
	}
	return db.Cleanup(ctx, time.Now())
}
