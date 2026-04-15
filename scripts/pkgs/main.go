// pkgs prints a space-separated list of packages in the current module,
// excluding any whose import path matches one of the given patterns.
//
// Usage: go run ./scripts/pkgs <pattern> [pattern ...]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pkgs <pattern> [pattern ...]")
		os.Exit(1)
	}

	args := os.Args[1:]
	for i, arg := range args {
		if arg == "." {
			// Resolve "." to the root package import path so callers don't
			// need to hard-code the module name (and thus the username).
			out, err := exec.Command("go", "list", ".").Output()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			root := strings.TrimSpace(string(out))
			// Anchor both ends so only the exact root package is excluded.
			args[i] = "^" + regexp.QuoteMeta(root) + "$"
		}
	}

	re := regexp.MustCompile(strings.Join(args, "|"))

	out, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var pkgs []string
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pkg != "" && !re.MatchString(pkg) {
			pkgs = append(pkgs, pkg)
		}
	}

	fmt.Print(strings.Join(pkgs, " "))
}
