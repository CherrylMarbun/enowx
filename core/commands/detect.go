// Package commands keeps the commands users run in a project again and
// again: found in its files (package.json's scripts, the Makefile's
// targets, Go, Cargo, Compose), saved by the user, and those run lately.
package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/enowx/enowx/core/checks"
	"github.com/enowx/enowx/proto"
)

// firstScripts are the scripts users run most, listed before the others.
var firstScripts = []string{"dev", "start", "serve", "build", "preview", "test", "lint"}

// Detect finds the commands of the project in dir.
func Detect(dir string) []proto.RunCommand {
	var out []proto.RunCommand
	add := func(source, name, command string) {
		out = append(out, proto.RunCommand{Name: name, Command: command, Source: source})
	}
	if scripts := nodeScripts(dir); len(scripts) > 0 {
		pm := checks.PackageManager(dir)
		for _, s := range scripts {
			add("package.json", s, pm+" run "+s)
		}
	}
	for _, t := range checks.MakeTargets(dir) {
		add("Makefile", "make "+t, "make "+t)
	}
	if exists(dir, "go.mod") {
		if exists(dir, "main.go") {
			add("go.mod", "go run .", "go run .")
		}
		add("go.mod", "go build", "go build ./...")
		add("go.mod", "go test", "go test ./...")
	}
	if exists(dir, "Cargo.toml") {
		add("Cargo.toml", "cargo run", "cargo run")
		add("Cargo.toml", "cargo build", "cargo build")
		add("Cargo.toml", "cargo test", "cargo test")
	}
	for _, f := range []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"} {
		if exists(dir, f) {
			add("compose", "compose up", "docker compose up")
			break
		}
	}
	return out
}

// nodeScripts are package.json's scripts, those users run most first.
func nodeScripts(dir string) []string {
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
	var out []string
	for _, s := range firstScripts {
		if _, ok := pkg.Scripts[s]; ok {
			out = append(out, s)
		}
	}
	var rest []string
	for s := range pkg.Scripts {
		if !slices.Contains(out, s) {
			rest = append(rest, s)
		}
	}
	slices.Sort(rest)
	return append(out, rest...)
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
