package matching

import (
	"playlistconverter/internal/domain"
	"testing"
)

func evaluationDataset() Dataset {
	return Dataset{SchemaVersion: 1, Split: "holdout", Provenance: "synthetic test fixture", IndependentLabels: false, Cases: []EvaluationCase{
		{ID: "one", Direction: "spotify-youtube", Family: "family-a", Source: song("s", "A Song"), Candidates: []domain.Song{song("a", "A Song")}, AcceptableIDs: []string{"a", "another"}},
		{ID: "two", Direction: "youtube-spotify", Family: "family-b", Source: song("s", "B Song"), Candidates: nil, NoMatch: true},
	}}
}
func TestEvaluate(t *testing.T) {
	d := evaluationDataset()
	report, err := Evaluate(d, .99, .5)
	if err != nil {
		t.Fatal(err)
	}
	m := report.Directions["spotify-youtube"]
	if m.Precision == nil || *m.Precision != 1 || m.Coverage != 1 || m.Correct != 1 {
		t.Fatalf("%+v", m)
	}
	if report.Passed {
		t.Fatal("synthetic data must not pass")
	}
	if m.PrecisionInterval[0] <= 0 || m.PrecisionInterval[1] > 1 {
		t.Fatal("invalid Wilson interval")
	}
	if report.Directions["youtube-spotify"].Precision != nil {
		t.Fatal("empty acceptance precision must be null")
	}
}
func TestEvaluateRejectsInvalidData(t *testing.T) {
	d := evaluationDataset()
	d.Cases[0].NoMatch = true
	if _, err := Evaluate(d, .99, .5); err == nil {
		t.Fatal("contradictory labels")
	}
	d = evaluationDataset()
	d.Cases[0].Direction = "wrong"
	if _, err := Evaluate(d, .99, .5); err == nil {
		t.Fatal("bad direction")
	}
}
func TestSplitLeakage(t *testing.T) {
	a, b := evaluationDataset(), evaluationDataset()
	a.Split = "calibration"
	if err := ValidateDisjoint(a, b); err == nil {
		t.Fatal("family leakage accepted")
	}
	b.Cases[0].Family = "other-a"
	b.Cases[1].Family = "other-b"
	b.Cases[0].Source.ID = "other-source-a"
	b.Cases[1].Source.ID = "other-source-b"
	if err := ValidateDisjoint(a, b); err != nil {
		t.Fatal(err)
	}
}

func TestDisjointRejectsRelabeledRecording(t *testing.T) {
	a, b := evaluationDataset(), evaluationDataset()
	a.Split = "calibration"
	b.Cases[0].Family = "different-label-a"
	b.Cases[1].Family = "different-label-b"
	if err := ValidateDisjoint(a, b); err == nil {
		t.Fatal("same provider recording leaked under different family")
	}
	a.Cases = nil
	if err := ValidateDisjoint(a, b); err == nil {
		t.Fatal("invalid calibration accepted")
	}
}
func TestDisjointRequiresCorrectSplit(t *testing.T) {
	a, b := evaluationDataset(), evaluationDataset()
	b.Cases[0].Family = "new-a"
	b.Cases[1].Family = "new-b"
	b.Cases[0].Source.ID = "new-a"
	b.Cases[1].Source.ID = "new-b"
	if err := ValidateDisjoint(a, b); err == nil {
		t.Fatal("holdout supplied as calibration")
	}
}
