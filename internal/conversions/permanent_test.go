package conversions

import (
	"context"
	"playlistconverter/internal/domain"
	"testing"
)

// refusingSource fails every source read with a fixed provider error.
type refusingSource struct {
	*fakeProvider
	err error
}

func (r refusingSource) ReadPlaylist(context.Context, domain.Token, string) (domain.Playlist, error) {
	return domain.Playlist{}, r.err
}

// Regression: a 403 while reading a Spotify playlist paused the conversion;
// every resume hit the same 403 and paused again, so the page flipped between
// "on hold" and "getting your tracklist" forever. Refusals that a retry cannot
// fix end the conversion while nothing has been written to the destination;
// temporary problems still pause.
func TestPermanentRefusalBeforeDestinationFails(t *testing.T) {
	for _, tc := range []struct {
		code         string
		status, err  string
		pause        string
		resumeAccept bool
	}{
		{code: "access_denied", status: "failed", err: "access_denied"},
		{code: "not_found", status: "failed", err: "not_found"},
		{code: "api_not_enabled", status: "failed", err: "api_not_enabled"},
		{code: "provider_rejected", status: "failed", err: "provider_rejected"},
		{code: "<html> https://x?token=SECRET", status: "failed", err: "other"},
		{code: "quota_exhausted", status: "reading", pause: "quota_exhausted", resumeAccept: true},
		{code: "provider_unavailable", status: "reading", pause: "provider_unavailable", resumeAccept: true},
		{code: "authorization_required", status: "reading", pause: "authorization_required", resumeAccept: true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			s, _ := setup(t)
			ctx := context.Background()
			src := s.Providers["spotify"].(*fakeProvider)
			s.Providers["spotify"] = refusingSource{src, &domain.ProviderError{Code: tc.code}}
			c, e := s.Create(ctx, "p", "https://open.spotify.com/playlist/0123456789012345678901", "k")
			if e != nil {
				t.Fatal(e)
			}
			_ = s.Step(ctx, c.ID)
			c, _ = s.Get(ctx, "p", c.ID)
			if c.Status != tc.status || c.Error != tc.err || c.PauseReason != tc.pause {
				t.Fatalf("status=%q error=%q pause=%q", c.Status, c.Error, c.PauseReason)
			}
			_, e = s.Resume(ctx, "p", c.ID)
			if (e == nil) != tc.resumeAccept {
				t.Fatalf("resume accepted=%v, want %v", e == nil, tc.resumeAccept)
			}
		})
	}
}
