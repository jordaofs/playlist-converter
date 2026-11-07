// Test-only loopback harness. This binary is excluded from the production image.
package main

import (
	"context"
	"crypto/rand"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"playlistconverter/internal/auth"
	"playlistconverter/internal/config"
	"playlistconverter/internal/conversions"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/httpapi"
	"playlistconverter/internal/providers/spotify"
	"playlistconverter/internal/providers/youtube"
	"playlistconverter/internal/store"
	fixtures "playlistconverter/internal/testfixtures/providers"
	"time"
)

func main() {
	f := fixtures.New()
	remote := httptest.NewServer(f)
	defer remote.Close()
	dir, e := os.MkdirTemp("", "converter-e2e-")
	if e != nil {
		log.Fatal(e)
	}
	defer os.RemoveAll(dir)
	db, e := store.Open(dir + "/test.db")
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	key := make([]byte, 32)
	rand.Read(key)
	client := &http.Client{Timeout: time.Second}
	adapters := map[string]domain.Provider{"spotify": spotify.New(client, remote.URL+"/v1"), "youtube": youtube.New(client, remote.URL+"/youtube/v3")}
	m := &auth.Manager{DB: db.DB, Key: key, KeyID: "test", Client: client, Providers: adapters, OAuth: map[string]*oauth2.Config{}}
	for _, p := range []string{"spotify", "youtube"} {
		m.OAuth[p] = &oauth2.Config{ClientID: "fixture", RedirectURL: "http://127.0.0.1:5173/api/auth/" + p + "/callback", Endpoint: oauth2.Endpoint{AuthURL: remote.URL + "/auth", TokenURL: remote.URL + "/token"}}
	}
	engine := conversions.New(db, adapters, m.Token)
	engine.Start(context.Background())
	api := (&httpapi.Server{Config: config.Config{BaseURL: "http://127.0.0.1:5173", AssetsPath: "web/dist"}, Auth: m, Engine: engine}).Handler()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /__test/scenario", f.Scenario)
	mux.Handle("/", api)
	log.Fatal(http.ListenAndServe("127.0.0.1:8081", mux))
}
