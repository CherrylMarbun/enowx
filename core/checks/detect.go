// Package checks finds and runs the commands that tell whether a
// session's work holds up: its tests, linters and build.
package checks

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// steps are the kinds of check, in the order they run: the quick ones
// first, so a failure shows soon.
var steps = []string{"lint", "typecheck", "build", "test"}

// Detect finds the checks of the project in dir: its Makefile's lint,
// build and test targets when it has them, else its language's usual
// commands.
func Detect(dir string) []proto.Check {
	if checks := makeChecks(dir); len(checks) > 0 {
		return checks
	}
	var out []proto.Check
	if exists(dir, "go.mod") {
		out = append(out,
			proto.Check{Name: "vet", Command: "go vet ./..."},
			proto.Check{Name: "build", Command: "go build ./..."},
			proto.Check{Name: "test", Command: "go test ./..."})
	}
	out = append(out, nodeChecks(dir)...)
	if exists(dir, "Cargo.toml") {
		out = append(out,
			proto.Check{Name: "build", Command: "cargo build"},
			proto.Check{Name: "test", Command: "cargo test"})
	}
	if exists(dir, "pytest.ini") || exists(dir, "pyproject.toml") && (exists(dir, "tests") || exists(dir, "test")) {
		out = append(out, proto.Check{Name: "test", Command: "pytest"})
	}
	return out
}

var makeTarget = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*:([^=]|$)`)

// makeChecks are the Makefile's targets named like checks.
func makeChecks(dir string) []proto.Check {
	targets := MakeTargets(dir)
	var out []proto.Check
	for _, step := range steps {
		if slices.Contains(targets, step) {
			out = append(out, proto.Check{Name: step, Command: "make " + step})
		}
	}
	return out
}

// nodeChecks are package.json's scripts named like checks, run with the
// package manager its lockfile names.
func nodeChecks(dir string) []proto.Check {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	pm := PackageManager(dir)
	var out []proto.Check
	for _, step := range steps {
		names := []string{step}
		if step == "typecheck" {
			names = []string{"typecheck", "type-check", "tsc"}
		}
		for _, name := range names {
			if _, ok := pkg.Scripts[name]; ok && !slices.ContainsFunc(out, func(c proto.Check) bool { return c.Name == step }) {
				out = append(out, proto.Check{Name: step, Command: pm + " run " + name})
			}
		}
	}
	return out
}

// PackageManager is the package manager of the Node project in dir, as its
// lockfile names it: npm, pnpm, yarn or bun.
func PackageManager(dir string) string {
	switch {
	case exists(dir, "pnpm-lock.yaml"):
		return "pnpm"
	case exists(dir, "yarn.lock"):
		return "yarn"
	case exists(dir, "bun.lockb") || exists(dir, "bun.lock"):
		return "bun"
	}
	return "npm"
}

// MakeTargets are the targets the Makefile in dir names, in its order.
func MakeTargets(dir string) []string {
	f, err := os.Open(filepath.Join(dir, "Makefile"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := makeTarget.FindStringSubmatch(sc.Text()); m != nil && !strings.HasPrefix(m[1], ".") && !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
