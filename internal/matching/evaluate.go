package matching

import (
	"fmt"
	"math"
	"playlistconverter/internal/domain"
)

type Dataset struct {
	SchemaVersion     int              `json:"schemaVersion"`
	Split             string           `json:"split"`
	Provenance        string           `json:"provenance"`
	IndependentLabels bool             `json:"independentLabels"`
	Cases             []EvaluationCase `json:"cases"`
}
type EvaluationCase struct {
	ID            string        `json:"id"`
	Direction     string        `json:"direction"`
	Family        string        `json:"family"`
	Source        domain.Song   `json:"source"`
	Candidates    []domain.Song `json:"candidates"`
	AcceptableIDs []string      `json:"acceptableIds"`
	NoMatch       bool          `json:"noMatch"`
}
type Metrics struct {
	Cases             int        `json:"cases"`
	Accepted          int        `json:"accepted"`
	Correct           int        `json:"correct"`
	Precision         *float64   `json:"precision"`
	PrecisionInterval [2]float64 `json:"precision95PercentInterval"`
	Coverage          float64    `json:"automaticCoverage"`
	CandidateRecall   float64    `json:"candidateRecall"`
	ReviewRate        float64    `json:"reviewRate"`
	UnavailableRate   float64    `json:"unavailableRate"`
}
type Report struct {
	MatcherVersion string             `json:"matcherVersion"`
	Passed         bool               `json:"passed"`
	Reasons        []string           `json:"reasons"`
	Directions     map[string]Metrics `json:"directions"`
}

func Evaluate(d Dataset, minPrecision, minCoverage float64) (Report, error) {
	if err := validateDataset(d); err != nil {
		return Report{}, err
	}
	if minPrecision <= 0 || minPrecision > 1 || minCoverage <= 0 || minCoverage > 1 || math.IsNaN(minPrecision) || math.IsNaN(minCoverage) {
		return Report{}, fmt.Errorf("quality thresholds must be in (0,1]")
	}
	report := Report{MatcherVersion: Version, Passed: true, Reasons: []string{}, Directions: map[string]Metrics{}}
	if !d.IndependentLabels {
		report.Reasons = append(report.Reasons, "Dataset does not contain independently verified labels")
	}
	if d.Split != "holdout" {
		report.Reasons = append(report.Reasons, "Release evaluation requires a held-out dataset")
	}
	for _, direction := range []string{"spotify-youtube", "youtube-spotify"} {
		metrics := evaluateDirection(d.Cases, direction)
		report.Directions[direction] = metrics
		if metrics.Cases < 100 {
			report.Reasons = append(report.Reasons, direction+": fewer than 100 held-out cases")
		}
		if metrics.Precision == nil || *metrics.Precision < minPrecision {
			report.Reasons = append(report.Reasons, direction+": accepted precision gate failed or is undefined")
		}
		if metrics.Coverage < minCoverage {
			report.Reasons = append(report.Reasons, direction+": automatic coverage gate failed")
		}
	}
	report.Passed = len(report.Reasons) == 0
	return report, nil
}
func validateDataset(d Dataset) error {
	if d.SchemaVersion != 1 || d.Provenance == "" || len(d.Cases) == 0 {
		return fmt.Errorf("dataset needs schemaVersion 1, provenance, and cases")
	}
	if d.Split != "holdout" && d.Split != "calibration" {
		return fmt.Errorf("unknown dataset split")
	}
	seen := map[string]bool{}
	for _, item := range d.Cases {
		if item.ID == "" || item.Family == "" || seen[item.ID] {
			return fmt.Errorf("case identifiers must be unique and recording families nonempty")
		}
		seen[item.ID] = true
		if item.Direction != "spotify-youtube" && item.Direction != "youtube-spotify" {
			return fmt.Errorf("case %s has an unknown direction", item.ID)
		}
		if item.NoMatch == (len(item.AcceptableIDs) > 0) {
			return fmt.Errorf("case %s needs either acceptable IDs or a no-match label", item.ID)
		}
		for _, id := range item.AcceptableIDs {
			if id == "" {
				return fmt.Errorf("case %s has an empty acceptable ID", item.ID)
			}
		}
	}
	return nil
}
func evaluateDirection(cases []EvaluationCase, direction string) Metrics {
	result := Metrics{}
	available, retrieved, reviews, unavailable, acceptedAvailable := 0, 0, 0, 0, 0
	for _, item := range cases {
		if item.Direction != direction {
			continue
		}
		result.Cases++
		if !item.NoMatch {
			available++
			if containsAny(item.Candidates, item.AcceptableIDs) {
				retrieved++
			}
		}
		decision, selected, _ := Decide(item.Source, Rank(item.Source, item.Candidates))
		switch decision {
		case "automatic":
			result.Accepted++
			if !item.NoMatch {
				acceptedAvailable++
			}
			if contains(item.AcceptableIDs, selected) {
				result.Correct++
			}
		case "review":
			reviews++
		case "unavailable":
			unavailable++
		}
	}
	if result.Accepted > 0 {
		p := float64(result.Correct) / float64(result.Accepted)
		result.Precision = &p
		result.PrecisionInterval = wilson(result.Correct, result.Accepted)
	}
	if available > 0 {
		result.Coverage = float64(acceptedAvailable) / float64(available)
		result.CandidateRecall = float64(retrieved) / float64(available)
	}
	if result.Cases > 0 {
		result.ReviewRate = float64(reviews) / float64(result.Cases)
		result.UnavailableRate = float64(unavailable) / float64(result.Cases)
	}
	return result
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func containsAny(songs []domain.Song, ids []string) bool {
	for _, song := range songs {
		if song.Unavailable == "" && contains(ids, song.ID) {
			return true
		}
	}
	return false
}
func wilson(success, total int) [2]float64 {
	n := float64(total)
	p := float64(success) / n
	z := 1.959963984540054
	denominator := 1 + z*z/n
	center := (p + z*z/(2*n)) / denominator
	half := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / denominator
	return [2]float64{math.Max(0, center-half), math.Min(1, center+half)}
}
func ValidateDisjoint(calibration, holdout Dataset) error {
	if err := validateDataset(calibration); err != nil {
		return fmt.Errorf("calibration: %w", err)
	}
	if err := validateDataset(holdout); err != nil {
		return fmt.Errorf("holdout: %w", err)
	}
	if calibration.Split != "calibration" || holdout.Split != "holdout" {
		return fmt.Errorf("expected calibration and holdout splits")
	}
	recordings := map[string]bool{}
	families := map[string]bool{}
	for _, item := range calibration.Cases {
		families[item.Family] = true
		if item.Source.ID != "" {
			recordings[item.Direction+":"+item.Source.ID] = true
		}
	}
	for _, item := range holdout.Cases {
		if item.Source.ID != "" && recordings[item.Direction+":"+item.Source.ID] {
			return fmt.Errorf("source recording %s occurs in calibration and holdout", item.Source.ID)
		}
		if families[item.Family] {
			return fmt.Errorf("recording family %s occurs in calibration and holdout", item.Family)
		}
	}
	return nil
}
