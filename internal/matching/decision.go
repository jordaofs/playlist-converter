package matching

import (
	"playlistconverter/internal/domain"
	"strings"
)

// Decide abstains whenever the strongest candidate contradicts known evidence.
func Decide(source domain.Song, candidates []domain.Candidate) (string, string, string) {
	if source.Unavailable != "" {
		return "unavailable", "", source.Unavailable
	}
	if len(candidates) == 0 {
		return "unavailable", "", "No available candidate found"
	}
	top := candidates[0]
	if source.Title == "" || len(source.Artists) == 0 || source.DurationMS <= 0 {
		return "review", "", "Source recording metadata is incomplete"
	}
	for _, reason := range top.Reasons {
		if strings.HasPrefix(reason, "Review:") {
			return "review", "", reason
		}
	}
	if top.Score < .95 {
		return "review", "", "Candidate evidence is not strong enough"
	}
	if len(candidates) > 1 && top.Score-candidates[1].Score < .08 {
		return "review", "", "Multiple candidates have similar evidence"
	}
	if top.Song.ID == "" {
		return "review", "", "Candidate identifier is missing"
	}
	return "automatic", top.Song.ID, "Title, artist, duration and recording-version evidence agree"
}
