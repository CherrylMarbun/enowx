package notes

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A picture taken out of a page stays: undo, a version kept, or another
// page may still name it. What nothing names any more goes once it has
// been left alone for assetsKept, rather than syncing to every host
// forever.

// assetsKept is how long a note's file no note names stays: one just
// added is named only once its page saves, and a version on another host
// may still name it a while.
const assetsKept = 7 * 24 * time.Hour

// SweepAssets removes the notes' files that no note, proposal or kept
// version of their note names and that haven't changed for assetsKept;
// it returns them, as "assets/<ID>/<file>".
func (s *Store) SweepAssets() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, err := os.ReadDir(filepath.Join(s.dir, "assets"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notes: %w", err)
	}
	// Every note names its files by their path: what the folder's notes
	// and proposals say, read once.
	var named strings.Builder
	tops, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("notes: %w", err)
	}
	for _, e := range tops {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".json")) {
			if data, err := os.ReadFile(filepath.Join(s.dir, e.Name())); err == nil {
				named.Write(data)
			}
		}
	}
	all := named.String()
	cutoff := time.Now().Add(-assetsKept)
	var removed []string
	for _, d := range ids {
		id := d.Name()
		if !d.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		files, err := os.ReadDir(s.AssetsDir(id))
		if err != nil {
			continue
		}
		versions := ""
		for _, st := range versionStamps(historyDir(s.dir, id)) {
			if data, err := os.ReadFile(filepath.Join(historyDir(s.dir, id), fmt.Sprintf("%d.md", st))); err == nil {
				versions += string(data)
			}
		}
		for _, f := range files {
			info, err := f.Info()
			if err != nil || f.IsDir() || info.ModTime().After(cutoff) {
				continue
			}
			ref := "assets/" + id + "/"
			if names := []string{ref + f.Name(), ref + url.PathEscape(f.Name())}; mentions(all, names) || mentions(versions, names) {
				continue
			}
			if os.Remove(filepath.Join(s.AssetsDir(id), f.Name())) == nil {
				removed = append(removed, ref+f.Name())
			}
		}
	}
	return removed, nil
}

func mentions(text string, names []string) bool {
	for _, n := range names {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}
