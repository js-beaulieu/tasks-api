// mincov exits 1 if total statement coverage in a Go coverage profile is below
// the given threshold.
//
// Usage: go run ./scripts/mincov <threshold> [coverage-file]
//
//	threshold     minimum coverage percentage, e.g. 75
//	coverage-file path to profile written by -coverprofile (default: coverage.out)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mincov <threshold> [coverage-file]")
		os.Exit(1)
	}

	threshold, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil || threshold < 0 || threshold > 100 {
		fmt.Fprintf(os.Stderr, "invalid threshold %q: must be a number between 0 and 100\n", os.Args[1])
		os.Exit(1)
	}

	file := "coverage.out"
	if len(os.Args) >= 3 {
		file = os.Args[2]
	}

	out, err := exec.Command("go", "tool", "cover", "-func="+file).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go tool cover: %v\n", err)
		os.Exit(1)
	}

	// The last line looks like: "total:    (statements)   72.3%"
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	last := lines[len(lines)-1]
	fields := strings.Fields(last)
	if len(fields) < 3 || fields[0] != "total:" {
		fmt.Fprintf(os.Stderr, "unexpected output from go tool cover: %q\n", last)
		os.Exit(1)
	}

	pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse coverage %q: %v\n", fields[len(fields)-1], err)
		os.Exit(1)
	}

	if pct < threshold {
		fmt.Fprintf(os.Stderr, "coverage %.1f%% is below minimum %.1f%%\n", pct, threshold)
		os.Exit(1)
	}

	fmt.Printf("coverage %.1f%% meets minimum %.1f%%\n", pct, threshold)
}
