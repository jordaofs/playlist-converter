package conversions

import "testing"

func TestParseSource(t *testing.T) {
	for _, c := range []struct{ url, p, id string }{
		{"https://open.spotify.com/playlist/0123456789012345678901?si=abc", "spotify", "0123456789012345678901"},
		{"https://www.youtube.com/playlist?list=PLabcdef_123", "youtube", "PLabcdef_123"},
		{"https://youtube.com/watch?v=abc&list=PLabc", "youtube", "PLabc"},
	} {
		p, id, err := ParseSource(c.url)
		if err != nil || p != c.p || id != c.id {
			t.Errorf("parse %s: %s %s %v", c.url, p, id, err)
		}
	}
	for _, s := range []string{"https://open.spotify.com.evil/playlist/abc", "http://open.spotify.com/playlist/abc", "https://u:p@youtube.com/playlist?list=PLabc", "https://youtube.com:444/playlist?list=PLabc", "https://youtube.com/playlist?list=PLabc&list=PLother", "https://youtu.be/abc", "https://youtube.com/playlist?list=WL", "https://open.spotify.com/playlist/short"} {
		if _, _, err := ParseSource(s); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
