package matching

import (
	"playlistconverter/internal/domain"
	"reflect"
	"testing"
)

func song(id, title string) domain.Song {
	return domain.Song{ID: id, Title: title, Artists: []string{"Artist"}, DurationMS: 180000}
}
func TestRankCandidates(t *testing.T) {
	source := song("source", "A Song")
	cases := []struct {
		name       string
		source     domain.Song
		candidates []domain.Song
		decision   string
	}{
		{"exact", source, []domain.Song{song("target", "A Song (Official Audio)")}, "automatic"},
		{"runner-up", source, []domain.Song{song("a", "A Song"), song("b", "A Song")}, "review"},
		{"live", source, []domain.Song{song("a", "A Song (Live)")}, "review"},
		{"cover", source, []domain.Song{song("a", "A Song Cover")}, "review"},
		{"remix", source, []domain.Song{song("a", "A Song Remix")}, "review"},
		{"speed", source, []domain.Song{song("a", "A Song (Sped Up)")}, "review"},
		{"remaster", source, []domain.Song{song("a", "A Song (Remastered)")}, "review"},
		{"unknown duration", domain.Song{ID: "s", Title: "A Song", Artists: []string{"Artist"}}, []domain.Song{song("a", "A Song")}, "review"},
		{"unknown artist", domain.Song{ID: "s", Title: "A Song", DurationMS: 180000}, []domain.Song{song("a", "A Song")}, "review"},
		{"no candidates", source, nil, "unavailable"},
		{"source unavailable", domain.Song{ID: "s", Unavailable: "deleted"}, []domain.Song{song("a", "A Song")}, "unavailable"},
		{"intro", source, []domain.Song{{ID: "a", Title: "A Song", Artists: []string{"Artist"}, DurationMS: 220000}}, "review"},
		{"wrong artist", source, []domain.Song{{ID: "a", Title: "A Song", Artists: []string{"Other"}, DurationMS: 180000}}, "review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]domain.Song(nil), tc.candidates...)
			ranked := Rank(tc.source, tc.candidates)
			decision, _, _ := Decide(tc.source, ranked)
			if decision != tc.decision {
				t.Fatalf("got %s, want %s; candidates=%+v", decision, tc.decision, ranked)
			}
			if !reflect.DeepEqual(original, tc.candidates) {
				t.Fatal("mutated input")
			}
		})
	}
}
func TestExplicitAndIdentifiers(t *testing.T) {
	yes, no := true, false
	s := song("s", "A Song")
	s.Explicit = &yes
	c := song("c", "A Song")
	c.Explicit = &no
	if d, _, _ := Decide(s, Rank(s, []domain.Song{c})); d != "review" {
		t.Fatal(d)
	}
	c.Explicit = nil
	if d, _, _ := Decide(s, Rank(s, []domain.Song{c})); d != "review" {
		t.Fatal("known versus unknown explicitness must review")
	}
	s.ISRC = "USAAA1234567"
	c.ISRC = "USBBB1234567"
	if d, _, _ := Decide(s, Rank(s, []domain.Song{c})); d != "review" {
		t.Fatal(d)
	}
}
func TestDeterministicDuplicateCandidates(t *testing.T) {
	s := song("s", "A Song")
	got := Rank(s, []domain.Song{song("b", "A Song"), song("a", "A Song"), song("b", "A Song")})
	if len(got) != 2 || got[0].Song.ID != "a" {
		t.Fatalf("%+v", got)
	}
}
func TestQualifierPreserved(t *testing.T) {
	if Normalize("Song (Live)") == Normalize("Song") {
		t.Fatal("erased live")
	}
	if Normalize("  CAFÉ — Official Audio ") != "café" {
		t.Fatal(Normalize("  CAFÉ — Official Audio "))
	}
}
func TestMalformedCandidateCannotSelect(t *testing.T) {
	s := song("s", "Song")
	c := song("", "Song")
	if len(Rank(s, []domain.Song{c})) != 0 {
		t.Fatal("empty identifier retained")
	}
}
func TestCanonicalUnicode(t *testing.T) {
	if Normalize("Cafe\u0301") != Normalize("Café") {
		t.Fatal("canonical Unicode forms differ")
	}
}

func TestTokenOrderIsNotRecordingIdentity(t *testing.T) {
	for _, tc := range []struct {
		name           string
		source, target domain.Song
	}{
		{"title ordering", song("s", "Love Me"), song("t", "Me Love")},
		{"artist name ordering", domain.Song{ID: "s", Title: "Song", Artists: []string{"John Paul"}, DurationMS: 180000}, domain.Song{ID: "t", Title: "Song", Artists: []string{"Paul John"}, DurationMS: 180000}},
		{"different artist segmentation", domain.Song{ID: "s", Title: "Song", Artists: []string{"Mary Jane"}, DurationMS: 180000}, domain.Song{ID: "t", Title: "Song", Artists: []string{"Mary", "Jane"}, DurationMS: 180000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if d, _, _ := Decide(tc.source, Rank(tc.source, []domain.Song{tc.target})); d != "review" {
				t.Fatalf("unproven equivalence automatically accepted: %s", d)
			}
		})
	}
}
func TestArtistOrderingAndCandidateIsolation(t *testing.T) {
	explicit := true
	s := domain.Song{ID: "s", Title: "Song", Artists: []string{"A", "B"}, DurationMS: 180000, Explicit: &explicit, ISRC: "abc"}
	target := domain.Song{ID: "t", Title: "Song", Artists: []string{"B", "A"}, DurationMS: 183000, Explicit: &explicit, ISRC: "ABC"}
	ranked := Rank(s, []domain.Song{target})
	if d, _, _ := Decide(s, ranked); d != "automatic" {
		t.Fatal(d)
	}
	ranked[0].Song.Artists[0] = "changed"
	*ranked[0].Song.Explicit = false
	if target.Artists[0] != "B" || !*target.Explicit {
		t.Fatal("candidate shares mutable evidence")
	}
}
