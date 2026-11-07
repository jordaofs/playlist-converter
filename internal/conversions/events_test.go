package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"playlistconverter/internal/domain"
	"strings"
	"testing"
)

func capture(s *Service) *bytes.Buffer {
	var out bytes.Buffer
	s.Log = slog.New(slog.NewJSONHandler(&out, nil))
	return &out
}

func events(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var list []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		list = append(list, e)
	}
	return list
}

// Private playlist content and identities must never reach the operator log.
var sentinels = []string{"Unique Song", "Artist", "SENTINEL_NAME", "PLdest", "spotify-owner", "youtube-owner", "0123456789012345678901", "SECRET"}

func assertPrivate(t *testing.T, out *bytes.Buffer) {
	t.Helper()
	for _, s := range sentinels {
		if strings.Contains(out.String(), s) {
			t.Fatalf("log leaked %q: %s", s, out.String())
		}
	}
	for _, e := range events(t, out) {
		for _, forbidden := range []string{"principal", "source_id", "name", "url", "token"} {
			if _, ok := e[forbidden]; ok {
				t.Fatalf("log has forbidden field %q", forbidden)
			}
		}
	}
}

func TestConversionLifecycleEvents(t *testing.T) {
	s, _ := setup(t)
	out := capture(s)
	ctx := context.Background()
	c := importJob(t, s)
	c, err := s.Publish(ctx, "p", c.ID, c.Revision, "SENTINEL_NAME", "publish-key")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err = s.Step(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
	}
	got := events(t, out)
	if len(got) != 2 || got[0]["msg"] != "conversion.started" || got[1]["msg"] != "conversion.completed" {
		t.Fatalf("unexpected events: %v", got)
	}
	done := got[1]
	if done["conversion_id"] != c.ID || done["direction"] != "spotify_to_youtube" || done["status"] != "completed" || done["items"] != float64(2) || done["published"] != float64(2) {
		t.Fatalf("completed event fields: %v", done)
	}
	if _, ok := done["duration_ms"].(float64); !ok {
		t.Fatalf("duration missing: %v", done)
	}
	assertPrivate(t, out)
}

func TestFailureAndPauseEvents(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	c, err := s.Create(ctx, "p", "https://open.spotify.com/playlist/0123456789012345678901", "key")
	if err != nil {
		t.Fatal(err)
	}
	c.SourceName = "SENTINEL_NAME"
	for _, tc := range []struct {
		name, msg, reason string
		run               func(domain.Conversion) error
	}{
		{"rejected", "conversion.failed", "playlist_not_owned", func(c domain.Conversion) error { return s.reject(ctx, c, "playlist_not_owned") }},
		{"provider pause", "conversion.paused", "rate_limited", func(c domain.Conversion) error {
			return s.handle(ctx, c, &domain.ProviderError{Code: "rate_limited"})
		}},
		{"unknown outcome", "conversion.paused", "unknown_remote_outcome", func(c domain.Conversion) error {
			return s.handle(ctx, c, &domain.ProviderError{Code: "network", UnknownOutcome: true})
		}},
		{"raw provider text is not a reason", "conversion.paused", "other", func(c domain.Conversion) error {
			return s.handle(ctx, c, &domain.ProviderError{Code: "<html> https://x?token=SECRET"})
		}},
		{"internal failure", "conversion.failed", "conversion_failed", func(c domain.Conversion) error {
			return s.handle(ctx, c, errors.New("SECRET database detail"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := capture(s)
			_ = tc.run(c)
			got := events(t, out)
			if len(got) != 1 || got[0]["msg"] != tc.msg || got[0]["reason"] != tc.reason {
				t.Fatalf("got %v", got)
			}
			assertPrivate(t, out)
		})
	}
}

func TestStepErrorsAreReportedWithoutFlooding(t *testing.T) {
	s, _ := setup(t)
	out := capture(s)
	busy := errors.New("database is locked")
	for i := 0; i < 50; i++ {
		s.reportStep("job", busy)
	}
	if n := len(events(t, out)); n != 1 {
		t.Fatalf("repeated error logged %d times", n)
	}
	s.reportStep("job", &domain.ProviderError{Code: "network"})
	s.reportStep("other-job", busy)
	if n := len(events(t, out)); n != 3 {
		t.Fatalf("changed error or other job not logged: %d", n)
	}
	s.reportStep("job", nil)
	s.reportStep("job", &domain.ProviderError{Code: "network"})
	got := events(t, out)
	if len(got) != 4 || got[3]["msg"] != "conversion.step_failed" || got[3]["error_code"] != "network" {
		t.Fatalf("recovery did not reset deduplication: %v", got)
	}
	if got[0]["error_code"] != "internal" || got[0]["error"] != "database is locked" {
		t.Fatalf("internal error detail missing: %v", got[0])
	}
}
