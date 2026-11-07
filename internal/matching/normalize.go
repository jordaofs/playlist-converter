package matching

import (
	"golang.org/x/text/unicode/norm"
	"regexp"
	"strings"
	"unicode"
)

var decoration = regexp.MustCompile(`\b(?:official (?:music video|video|audio|lyric video|lyrics)|lyrics? video|visuali[sz]er|hd|4k)\b`)
var variants = []string{"live", "remix", "remastered", "remaster", "acoustic", "cover", "instrumental", "karaoke", "sped up", "slowed", "nightcore", "radio edit", "extended", "clean", "explicit"}

// Normalize removes presentation noise, never recording-version qualifiers.
func Normalize(value string) string {
	value = strings.ToLower(norm.NFC.String(value))
	value = decoration.ReplaceAllString(value, " ")
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return ' '
	}, value)
	return strings.Join(strings.Fields(value), " ")
}
func qualifiers(value string) string {
	normalized := " " + Normalize(value) + " "
	var found []string
	for _, variant := range variants {
		if strings.Contains(normalized, " "+variant+" ") {
			found = append(found, variant)
		}
	}
	return strings.Join(found, "|")
}
func similarity(left, right string) float64 {
	if left == "" || right == "" {
		return 0
	}
	if left == right {
		return 1
	}
	a, b := strings.Fields(left), strings.Fields(right)
	set := map[string]bool{}
	for _, v := range a {
		set[v] = true
	}
	seen := map[string]bool{}
	overlap := 0
	for _, v := range b {
		if set[v] && !seen[v] {
			overlap++
		}
		seen[v] = true
	}
	union := len(set)
	for v := range seen {
		if !set[v] {
			union++
		}
	}
	// Token overlap helps rank near matches, but reordered words are not exact evidence.
	score := float64(overlap) / float64(union)
	if score == 1 {
		return .9
	}
	return score
}
