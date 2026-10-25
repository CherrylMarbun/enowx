package notes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// A note's earlier versions, kept per host in .history/<id>/<unix nanos>.md
// beside the notes: what it was before a change, at most one per
// versionEvery while the user types, always before a change from
// elsewhere (an accepted proposal, a sync, a restore), versionsKept at
// most.

const (
	versionEvery = 10 * time.Minute
	versionsKept = 50
)

func historyDir(dir, id string) string { return filepath.Join(dir, ".history", id) }

// KeepVersion keeps data, note id's file about to be replaced in notes
// folder dir, as a version: always when force, else when the last kept
// is older than versionEvery.
func KeepVersion(dir, id string, data []byte, force bool) error {
	if !idPattern.MatchString(id) || len(data) == 0 {
		return nil
	}
	hd := historyDir(dir, id)
	stamps := versionStamps(hd)
	now := time.Now()
	if !force && len(stamps) > 0 && now.Sub(time.Unix(0, stamps[len(stamps)-1])) < versionEvery {
		return nil
	}
	if len(stamps) > 0 {
		if last, err := os.ReadFile(filepath.Join(hd, fmt.Sprintf("%d.md", stamps[len(stamps)-1]))); err == nil && string(last) == string(data) {
			return nil
		}
	}
	if err := os.MkdirAll(hd, 0o700); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	if err := os.WriteFile(filepath.Join(hd, fmt.Sprintf("%d.md", now.UnixNano())), data, 0o600); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	for _, old := range stamps[:max(0, len(stamps)+1-versionsKept)] {
		os.Remove(filepath.Join(hd, fmt.Sprintf("%d.md", old)))
	}
	return nil
}

func versionStamps(hd string) []int64 {
	entries, _ := os.ReadDir(hd)
	var out []int64
	for _, e := range entries {
		if n, err := strconv.ParseInt(strings.TrimSuffix(e.Name(), ".md"), 10, 64); err == nil {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

// keep keeps note id's file as it is now as a version; s.mu is held.
func (s *Store) keep(id string, force bool) {
	path, err := s.file(id)
	if err != nil {
		return
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = KeepVersion(s.dir, id, data, force)
	}
}

// History lists note id's earlier versions, the latest first.
func (s *Store) History(id string) ([]proto.NoteVersion, error) {
	if _, err := s.file(id); err != nil {
		return nil, err
	}
	stamps := versionStamps(historyDir(s.dir, id))
	out := []proto.NoteVersion{}
	for i := len(stamps) - 1; i >= 0; i-- {
		ver := strconv.FormatInt(stamps[i], 10)
		v, err := s.Version(id, ver)
		if err != nil {
			continue
		}
		out = append(out, proto.NoteVersion{ID: ver, Time: time.Unix(0, stamps[i]).UTC(), Title: v.Title, Size: len(v.Body)})
	}
	return out, nil
}

// Version is version ver of note id.
func (s *Store) Version(id, ver string) (proto.Note, error) {
	if _, err := s.file(id); err != nil {
		return proto.Note{}, err
	}
	if _, err := strconv.ParseInt(ver, 10, 64); err != nil {
		return proto.Note{}, fmt.Errorf("notes: no version %q", ver)
	}
	path := filepath.Join(historyDir(s.dir, id), ver+".md")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return proto.Note{}, fmt.Errorf("notes: no version %q of %s", ver, id)
	}
	if err != nil {
		return proto.Note{}, fmt.Errorf("notes: %w", err)
	}
	defer f.Close()
	return parse(id, path, f, true)
}

// Restore brings version ver of note id back: its title, icon and body,
// what it is now kept as a version first.
func (s *Store) Restore(id, ver string) (proto.NoteInfo, error) {
	v, err := s.Version(id, ver)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	s.mu.Lock()
	s.keep(id, true)
	s.mu.Unlock()
	cur, err := s.Get(id)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	cur.Title, cur.Icon, cur.Body = v.Title, v.Icon, v.Body
	return s.Save(cur)
}
