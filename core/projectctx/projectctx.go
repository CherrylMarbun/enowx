// Package projectctx tells an agent planning for a project what it is,
// briefly: its stack, branch and last commit, its README's start and its
// folders, and reads its files, read-only, when the agent asks for one.
// Nothing here writes to the project.
package projectctx

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/files"
	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// Limits keep a context small: an agent reads the files it needs after.
const (
	readmeMax = 4000
	treeMax   = 120
	fileMax   = 64 << 10
)

// Of is the context of the project in folder dir.
func Of(dir string) (proto.ProjectContext, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return proto.ProjectContext{}, fmt.Errorf("project: %w", err)
	}
	if !fi.IsDir() {
		return proto.ProjectContext{}, fmt.Errorf("project: %s is not a folder", dir)
	}
	c := proto.ProjectContext{Name: filepath.Base(dir), Path: dir, Taken: time.Now().UTC().Truncate(time.Second)}
	if remote, err := git.Run(dir, "config", "--get", "remote.origin.url"); err == nil {
		c.Remote = remote
	}
	c.Branch = git.CurrentBranch(dir)
	if last, err := git.Run(dir, "log", "-1", "--format=%h %s (%cr)"); err == nil {
		c.LastCommit = last
	}
	c.Stack = stack(dir)
	c.Readme = readme(dir)
	paths, _ := files.Find(dir, "", 5000)
	c.Tree = tree(paths)
	return c, nil
}

// Read reads file rel of project dir, read-only: text, cut at fileMax;
// binary files and paths out of the folder are refused.
func Read(dir, rel string) (proto.ProjectFile, error) {
	fc, err := files.Read(dir, rel, time.Time{})
	if err != nil {
		return proto.ProjectFile{}, err
	}
	if fc.Binary || fc.Image != "" {
		return proto.ProjectFile{}, fmt.Errorf("project: %s isn't text", rel)
	}
	out := proto.ProjectFile{File: rel, Text: fc.Text, Size: fc.Size, Truncated: fc.Truncated}
	if len(out.Text) > fileMax {
		out.Text, out.Truncated = out.Text[:fileMax], true
	}
	return out, nil
}

// markers name a stack by a file at the project's top.
var markers = []struct{ file, name string }{
	{"go.mod", "Go"}, {"package.json", "Node.js"}, {"tsconfig.json", "TypeScript"}, {"Cargo.toml", "Rust"},
	{"pyproject.toml", "Python"}, {"requirements.txt", "Python"}, {"Gemfile", "Ruby"}, {"composer.json", "PHP"},
	{"pom.xml", "Java (Maven)"}, {"build.gradle", "Gradle"}, {"build.gradle.kts", "Gradle"}, {"pubspec.yaml", "Flutter/Dart"},
	{"Package.swift", "Swift"}, {"mix.exs", "Elixir"}, {"Dockerfile", "Docker"}, {"docker-compose.yml", "Docker Compose"},
	{"next.config.js", "Next.js"}, {"next.config.mjs", "Next.js"}, {"vite.config.ts", "Vite"}, {"tailwind.config.js", "Tailwind"},
	{"tailwind.config.ts", "Tailwind"}, {"components.json", "shadcn/ui"}, {"artisan", "Laravel"}, {"manage.py", "Django"},
}

func stack(dir string) []string {
	var out []string
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil && !slices.Contains(out, m.name) {
			out = append(out, m.name)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		for _, dep := range []string{"react", "vue", "svelte", "@angular/core", "express", "electron", "@tauri-apps/api"} {
			if strings.Contains(string(data), `"`+dep+`"`) {
				out = append(out, dep)
			}
		}
	}
	return out
}

func readme(dir string) string {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(strings.ToLower(e.Name()), "readme") && !e.IsDir() {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return ""
			}
			if len(data) > readmeMax {
				return string(data[:readmeMax]) + "\n…"
			}
			return string(data)
		}
	}
	return ""
}

// tree is the project's folders two deep, with how many files each holds,
// and the files at its top.
func tree(paths []string) []string {
	count := map[string]int{}
	var top []string
	for _, p := range paths {
		parts := strings.Split(p, "/")
		switch len(parts) {
		case 1:
			top = append(top, p)
		case 2:
			count[parts[0]+"/"]++
		default:
			count[parts[0]+"/"]++
			count[path.Join(parts[0], parts[1])+"/"]++
		}
	}
	var dirs []string
	for d := range count {
		dirs = append(dirs, d)
	}
	slices.Sort(dirs)
	slices.Sort(top)
	var out []string
	for _, d := range dirs {
		out = append(out, fmt.Sprintf("%s (%d files)", d, count[d]))
	}
	out = append(out, top...)
	if len(out) > treeMax {
		out = append(out[:treeMax], fmt.Sprintf("… and %d more", len(out)-treeMax))
	}
	return out
}
