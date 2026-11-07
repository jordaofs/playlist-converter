package matching

import (
	"math"
	"playlistconverter/internal/domain"
	"sort"
	"strings"
)

const Version = "conservative-v1-experimental"

// Rank returns a new, deterministically ordered candidate slice. Scores are not probabilities.
func Rank(source domain.Song, songs []domain.Song) []domain.Candidate {
	result := make([]domain.Candidate, 0, len(songs))
	seen := map[string]bool{}
	for _, song := range songs {
		if song.ID == "" || song.Unavailable != "" || seen[song.ID] {
			continue
		}
		seen[song.ID] = true
		candidate := score(source, song)
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].Song.ID < result[j].Song.ID
		}
		return result[i].Score > result[j].Score
	})
	return result
}
func score(source, target domain.Song) domain.Candidate {
	title := similarity(Normalize(source.Title), Normalize(target.Title))
	artist := artistSimilarity(source.Artists, target.Artists)
	duration := durationSimilarity(source.DurationMS, target.DurationMS)
	score := 0.50*title + 0.30*artist + 0.20*duration
	reasons := []string{}
	if title == 1 {
		reasons = append(reasons, "Title agrees")
	} else {
		reasons = append(reasons, "Review: title differs")
	}
	if artist == 1 {
		reasons = append(reasons, "Artist metadata agrees")
	} else {
		reasons = append(reasons, "Review: artist metadata differs or is missing")
	}
	if duration == 1 {
		reasons = append(reasons, "Duration agrees")
	} else if duration == 0 {
		reasons = append(reasons, "Review: duration differs or is missing")
	}
	if qualifiers(source.Title) != qualifiers(target.Title) {
		score = math.Min(score, .50)
		reasons = append(reasons, "Review: recording version differs")
	}
	if source.Explicit != nil && target.Explicit != nil && *source.Explicit != *target.Explicit {
		score = math.Min(score, .50)
		reasons = append(reasons, "Review: explicit/clean mismatch")
	}
	if (source.Explicit == nil) != (target.Explicit == nil) {
		score = math.Min(score, .89)
		reasons = append(reasons, "Review: explicitness is unknown on one service")
	}
	if source.ISRC != "" && target.ISRC != "" {
		if strings.EqualFold(source.ISRC, target.ISRC) {
			reasons = append(reasons, "Recording identifier agrees")
		} else {
			score = math.Min(score, .50)
			reasons = append(reasons, "Review: recording identifiers differ")
		}
	}
	// Cloning nested artist evidence prevents a caller changing source input through results.
	target.Artists = append([]string(nil), target.Artists...)
	if target.Explicit != nil {
		value := *target.Explicit
		target.Explicit = &value
	}
	return domain.Candidate{Song: target, Score: score, Reasons: reasons}
}
func artistSimilarity(left, right []string) float64 {
	a, b := make([]string, 0, len(left)), make([]string, 0, len(right))
	for _, value := range left {
		if value = Normalize(value); value != "" {
			a = append(a, value)
		}
	}
	for _, value := range right {
		if value = Normalize(value); value != "" {
			b = append(b, value)
		}
	}
	sort.Strings(a)
	sort.Strings(b)
	// Preserve boundaries between artists; a solo name must not equal two artists.
	if len(a) == len(b) && strings.Join(a, "\x00") == strings.Join(b, "\x00") && len(a) > 0 {
		return 1
	}
	return math.Min(.9, similarity(strings.Join(a, " "), strings.Join(b, " ")))
}
func durationSimilarity(left, right int) float64 {
	if left <= 0 || right <= 0 {
		return 0
	}
	gap := math.Abs(float64(left - right))
	if gap <= 2000 {
		return 1
	}
	if gap <= 5000 {
		return .8
	}
	return 0
}
