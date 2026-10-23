package files

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/transfer"
)

// Quick open finds files by a few letters of their path, as editors do:
// the letters in order, anywhere in it, best where they start words of
// the file's name.

// maxListed bounds the files a folder lists for finding.
const maxListed = 200_000

// listing is a folder's files, kept a moment: finding runs on every key.
type listing struct {
	at    time.Time
	paths []string
}

var listings struct {
	sync.Mutex
	byRoot map[string]listing
}

// listFor is how long a folder's files are kept.
const listFor = 5 * time.Second

// Find returns up to limit files of root whose paths match query, best
// first: the files git tracks or would (ignored ones left out) in a
// repository, else every file outside folders of dependencies and builds.
func Find(root, query string, limit int) ([]string, error) {
	paths, err := listed(root)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.ReplaceAll(query, " ", ""))
	type hit struct {
		path  string
		score int
	}
	var hits []hit
	for _, p := range paths {
		if s, ok := score(p, q); ok {
			hits = append(hits, hit{p, s})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return len(a.path) - len(b.path)
	})
	out := make([]string, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.path)
	}
	return out, nil
}

func listed(root string) ([]string, error) {
	listings.Lock()
	if l, ok := listings.byRoot[root]; ok && time.Since(l.at) < listFor {
		listings.Unlock()
		return l.paths, nil
	}
	listings.Unlock()
	paths, err := list(root)
	if err != nil {
		return nil, err
	}
	listings.Lock()
	if listings.byRoot == nil {
		listings.byRoot = map[string]listing{}
	}
	listings.byRoot[root] = listing{time.Now(), paths}
	listings.Unlock()
	return paths, nil
}

func list(root string) ([]string, error) {
	if _, err := git.TopLevel(root); err == nil {
		out, err := git.Run(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		if err == nil {
			paths := strings.Split(strings.TrimRight(out, "\x00"), "\x00")
			paths = slices.DeleteFunc(paths, func(p string) bool { return p == "" })
			slices.Sort(paths)
			paths = slices.Compact(paths) // listed as cached and other when changed
			return paths[:min(len(paths), maxListed)], nil
		}
	}
	var paths []string
	skip := append(slices.Clone(transfer.Skipped), ".git")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && slices.Contains(skip, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err == nil {
			paths = append(paths, filepath.ToSlash(rel))
		}
		if len(paths) >= maxListed {
			return filepath.SkipAll
		}
		return nil
	})
	return paths, err
}

// score rates how well p matches q, its letters in order: higher for
// letters in the file's name, at the start of words, one after another.
func score(p, q string) (int, bool) {
	if q == "" {
		return 0, true
	}
	lower := strings.ToLower(p)
	name := strings.LastIndexByte(p, '/') + 1
	s, i, prev := 0, 0, -2
	for j := 0; j < len(lower) && i < len(q); j++ {
		if lower[j] != q[i] {
			continue
		}
		s++
		if j >= name {
			s += 2
		}
		if j == 0 || !unicode.IsLetter(rune(p[j-1])) && !unicode.IsDigit(rune(p[j-1])) || unicode.IsUpper(rune(p[j])) && unicode.IsLower(rune(p[j-1])) {
			s += 3 // the start of a word
		}
		if prev == j-1 {
			s += 4
		}
		prev = j
		i++
	}
	if i < len(q) {
		return 0, false
	}
	if strings.Contains(strings.ToLower(p[name:]), q) {
		s += 20
	}
	return s, true
}
