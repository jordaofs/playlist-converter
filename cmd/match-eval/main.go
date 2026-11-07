// match-eval evaluates frozen matching behavior. Synthetic fixtures never pass release gates.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"playlistconverter/internal/matching"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("match-eval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataset := flags.String("dataset", "internal/testfixtures/matching/holdout.json", "Held-out labeled dataset")
	calibration := flags.String("calibration", "", "Calibration dataset (defaults to calibration.json beside holdout)")
	precision := flags.Float64("min-precision", .99, "Minimum accepted-match precision")
	coverage := flags.Float64("min-coverage", .5, "Minimum automatic coverage of available recordings")
	output := flags.String("output", "", "Optional JSON report file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	heldout, err := readDataset(*dataset)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	path := *calibration
	if path == "" {
		path = filepath.Join(filepath.Dir(*dataset), "calibration.json")
	}
	training, err := readDataset(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err = matching.ValidateDisjoint(training, heldout); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, err := matching.Evaluate(heldout, *precision, *coverage)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	raw = append(raw, '\n')
	if *output != "" {
		if err = os.MkdirAll(filepath.Dir(*output), 0700); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err = os.WriteFile(*output, raw, 0600); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if _, err = stdout.Write(raw); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !report.Passed {
		return 1
	}
	return 0
}
func readDataset(path string) (matching.Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return matching.Dataset{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 20<<20))
	decoder.DisallowUnknownFields()
	var dataset matching.Dataset
	if err = decoder.Decode(&dataset); err != nil {
		return dataset, fmt.Errorf("read %s: %w", path, err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return dataset, fmt.Errorf("read %s: unexpected trailing content", path)
	}
	return dataset, nil
}
