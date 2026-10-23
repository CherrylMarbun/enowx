package envsync

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// Skills come from a git repository too: one is cloned, the folders
// holding a SKILL.md are offered, and those picked are added.

// maxFetchDepth bounds how deep a repository is searched for skills.
const maxFetchDepth = 5

// repoURL splits a repository URL as a browser shows it: GitHub's
// .../tree/<branch>/<folder> names a branch and a folder in it.
func repoURL(raw string) (clone, branch, folder string, err error) {
	raw = strings.TrimSpace(raw)
	u, perr := url.Parse(raw)
	if perr == nil && u.Scheme == "file" {
		return raw, "", "", nil
	}
	if perr != nil || u.Host == "" {
		if strings.Contains(raw, "@") && strings.Contains(raw, ":") { // git@host:owner/repo
			return raw, "", "", nil
		}
		return "", "", "", fmt.Errorf("envsync: %q isn't a repository's address", raw)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Host == "github.com" && len(parts) >= 2 {
		clone = "https://github.com/" + parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
		if len(parts) >= 4 && (parts[2] == "tree" || parts[2] == "blob") {
			branch, folder = parts[3], strings.Join(parts[4:], "/")
		}
		return clone, branch, strings.TrimSuffix(folder, "/SKILL.md"), nil
	}
	return raw, "", "", nil
}

// FetchSkills clones the repository at raw into dir, which must not exist,
// and lists the skills in it (or in the folder its address names).
func FetchSkills(raw, dir string) ([]proto.FoundSkill, error) {
	clone, branch, folder, err := repoURL(raw)
	if err != nil {
		return nil, err
	}
	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	if _, err := git.Run("", append(args, clone, dir)...); err != nil {
		return nil, fmt.Errorf("envsync: clone %s: %w", clone, err)
	}
	root := filepath.Join(dir, filepath.FromSlash(folder))
	var found []proto.FoundSkill
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || strings.Count(rel, string(filepath.Separator)) >= maxFetchDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "SKILL.md" {
			return nil
		}
		data, _ := os.ReadFile(p)
		skill := proto.FoundSkill{Path: filepath.ToSlash(filepath.Dir(rel)), Name: path.Base(filepath.ToSlash(filepath.Dir(p)))}
		if skill.Path == "." {
			skill.Name = path.Base(strings.TrimSuffix(clone, ".git"))
		}
		skill.Name, skill.Description = frontMatterField(data, "name", skill.Name), frontMatterField(data, "description", "")
		found = append(found, skill)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("envsync: %s: %w", raw, err)
	}
	if len(found) == 0 {
		return nil, errors.New("envsync: no SKILL.md in it")
	}
	return found, nil
}

// SkillFrom is the payload that adds the skill in folder rel of a cloned
// repository dir to agent, named as its SKILL.md says, else as its folder.
func SkillFrom(dir, rel, agent string, sc Scope) (proto.EnvPayload, error) {
	root := filepath.Join(dir, filepath.FromSlash(rel))
	if !strings.HasPrefix(root, filepath.Clean(dir)) {
		return proto.EnvPayload{}, fmt.Errorf("envsync: %s leaves the repository", rel)
	}
	data, _ := os.ReadFile(filepath.Join(root, "SKILL.md"))
	name := frontMatterField(data, "name", filepath.Base(root))
	var files []proto.EnvFile
	total := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if total += len(data); total > maxItemBytes {
			return fmt.Errorf("more than %d MB", maxItemBytes>>20)
		}
		info, _ := d.Info()
		r, _ := filepath.Rel(root, p)
		files = append(files, proto.EnvFile{Path: filepath.ToSlash(r), Mode: uint32(info.Mode().Perm()), Data: data})
		return nil
	})
	if err != nil {
		return proto.EnvPayload{}, fmt.Errorf("envsync: skill %s: %w", name, err)
	}
	return NewSkillIn(agent, name, files, sc)
}

// frontMatterField reads a field of a Markdown file's front matter.
func frontMatterField(data []byte, key, fallback string) string {
	text := string(data)
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return fallback
	}
	head, _, _ := strings.Cut(rest, "\n---")
	lines := strings.Split(head, "\n")
	for i, line := range lines {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) != key || strings.HasPrefix(line, " ") {
			continue
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, ">") || strings.HasPrefix(v, "|") {
			// A block: the indented lines after it, as one line.
			var parts []string
			for _, next := range lines[i+1:] {
				if next != "" && !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
					break
				}
				if t := strings.TrimSpace(next); t != "" {
					parts = append(parts, t)
				}
			}
			v = strings.Join(parts, " ")
		}
		if v = strings.Trim(v, `"'`); v != "" {
			return v
		}
	}
	return fallback
}
