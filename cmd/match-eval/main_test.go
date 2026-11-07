package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/matching"
	"testing"
)

func writeData(t *testing.T, path string, d matching.Dataset) {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func fixture(split string) matching.Dataset {
	return matching.Dataset{SchemaVersion: 1, Split: split, Provenance: "unit test", Cases: []matching.EvaluationCase{{ID: "one", Direction: "spotify-youtube", Family: split, Source: domain.Song{ID: "s-" + split, Title: "Song", Artists: []string{"Artist"}, DurationMS: 180000}, Candidates: []domain.Song{{ID: "t", Title: "Song", Artists: []string{"Artist"}, DurationMS: 180000}}, AcceptableIDs: []string{"t"}}}}
}
func TestRunSyntheticGate(t *testing.T) {
	dir := t.TempDir()
	writeData(t, filepath.Join(dir, "calibration.json"), fixture("calibration"))
	path := filepath.Join(dir, "holdout.json")
	writeData(t, path, fixture("holdout"))
	var out, errs bytes.Buffer
	report := filepath.Join(dir, "reports", "out.json")
	code := run([]string{"-dataset", path, "-output", report}, &out, &errs)
	if code != 1 {
		t.Fatalf("%d: %s", code, errs.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"passed": false`)) {
		t.Fatal(out.String())
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatal(err)
	}
}
func TestRunErrorPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "holdout.json")
	writeData(t, path, fixture("holdout"))
	var out, errs bytes.Buffer
	for _, args := range [][]string{{"-unknown"}, {"-dataset", filepath.Join(dir, "absent.json")}, {"-dataset", path}} {
		if got := run(args, &out, &errs); got != 2 {
			t.Fatalf("%v: %d", args, got)
		}
	}
	writeData(t, filepath.Join(dir, "calibration.json"), fixture("holdout"))
	if run([]string{"-dataset", path}, &out, &errs) != 2 {
		t.Fatal("leakage accepted")
	}
	writeData(t, filepath.Join(dir, "calibration.json"), fixture("calibration"))
	if run([]string{"-dataset", path, "-min-precision", "2"}, &out, &errs) != 2 {
		t.Fatal("invalid threshold accepted")
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if run([]string{"-dataset", path, "-output", filepath.Join(blocker, "out.json")}, &out, &errs) != 2 {
		t.Fatal("bad output dir accepted")
	}
	if run([]string{"-dataset", path, "-output", dir}, &out, &errs) != 2 {
		t.Fatal("directory output accepted")
	}
}
func TestReadDatasetStrict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	for _, raw := range []string{`{"unexpected":true}`, `{} {}`, `invalid`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readDataset(path); err == nil {
			t.Fatal("invalid data accepted")
		}
	}
}
