package notes

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// Notes brought in whole, from an import or a backup, and the files they
// show (images, attachments), kept in assets/<note ID>/ beside the notes
// and named from their bodies as "assets/<note ID>/<file>".

// Dir is the folder the notes are kept in.
func (s *Store) Dir() string { return s.dir }

// AssetsDir is where note id's files are kept.
func (s *Store) AssetsDir(id string) string { return filepath.Join(s.dir, "assets", id) }

// Restore writes n as it is: under its ID when that is a note's and free
// (a new one otherwise), its parent, group (made if missing), order (last
// among its siblings when negative) and times (now when zero).
func (s *Store) Restore(n proto.Note) (proto.NoteInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.Title = strings.Join(strings.Fields(n.Title), " ")
	n.Icon, n.IconReview = broughtIcon(strings.TrimSpace(strings.ReplaceAll(n.Icon, "\n", "")), n.IconReview)
	if n.ID != "" {
		if _, err := s.file(n.ID); err != nil {
			n.ID = ""
		} else if _, err := s.read(n.ID, false); err == nil {
			n.ID = ""
		}
	}
	if n.ID == "" {
		var b [6]byte
		rand.Read(b[:])
		n.ID = hex.EncodeToString(b[:])
	}
	if n.Parent != "" {
		if _, err := s.read(n.Parent, false); err != nil {
			return proto.NoteInfo{}, err
		}
		n.Group = ""
	}
	if n.Group != "" {
		if err := s.ensureGroup(n.Group); err != nil {
			return proto.NoteInfo{}, err
		}
	}
	if n.Order < 0 {
		list, err := s.List()
		if err != nil {
			return proto.NoteInfo{}, err
		}
		n.Order = 0
		for _, o := range list {
			if o.Parent == n.Parent && o.Order >= n.Order {
				n.Order = o.Order + 1
			}
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	if n.Created.IsZero() {
		n.Created = now
	}
	if n.Updated.IsZero() {
		n.Updated = n.Created
	}
	n.Created, n.Updated = n.Created.UTC().Truncate(time.Second), n.Updated.UTC().Truncate(time.Second)
	if err := s.write(n); err != nil {
		return proto.NoteInfo{}, err
	}
	return n.NoteInfo, nil
}

// Replace writes note n.ID's title, icon, body and times as given,
// keeping its place: an import made again over what it made before.
func (s *Store) Replace(n proto.Note) (proto.NoteInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.read(n.ID, false)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	n.Parent, n.Group, n.Order, n.Created = old.Parent, old.Group, old.Order, old.Created
	if len(n.Projects) == 0 {
		n.Projects = old.Projects
	}
	n.Icon, n.IconReview = broughtIcon(n.Icon, n.IconReview)
	if n.Updated.IsZero() {
		n.Updated = time.Now().UTC()
	}
	n.Updated = n.Updated.UTC().Truncate(time.Second)
	if err := s.write(n); err != nil {
		return proto.NoteInfo{}, err
	}
	return n.NoteInfo, nil
}

// SaveAsset keeps data as note id's file name, a free name when taken by
// other data, and returns the path its body names it by.
func (s *Store) SaveAsset(id, name string, data []byte) (string, error) {
	if _, err := s.file(id); err != nil {
		return "", err
	}
	name = assetName(name)
	dir := s.AssetsDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("notes: %w", err)
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		path := filepath.Join(dir, name)
		old, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(path, data, 0o600); err != nil {
				return "", fmt.Errorf("notes: %w", err)
			}
			break
		}
		if err == nil && string(old) == string(data) {
			break
		}
		name = fmt.Sprintf("%s-%d%s", base, i+1, ext)
	}
	return "assets/" + id + "/" + name, nil
}

// Asset reads a file a note's body names ("assets/<note ID>/<file>").
func (s *Store) Asset(ref string) ([]byte, error) {
	rest, ok := strings.CutPrefix(ref, "assets/")
	id, name, ok2 := strings.Cut(rest, "/")
	if !ok || !ok2 || !idPattern.MatchString(id) || name != assetName(name) {
		return nil, fmt.Errorf("notes: no file %q", ref)
	}
	data, err := os.ReadFile(filepath.Join(s.AssetsDir(id), name))
	if err != nil {
		return nil, fmt.Errorf("notes: %w", err)
	}
	return data, nil
}

// assetName is name made safe as a file's in a note's folder.
func assetName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		switch {
		case r == ' ':
			return '-' // a Markdown link's path takes no spaces
		case r < ' ' || strings.ContainsRune(`/\:*?"<>|()[]`, r):
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	return name
}

// broughtIcon is the icon a note brought from elsewhere keeps: an SVG
// icon as it is, an emoji's closest one (review when none is close).
func broughtIcon(icon string, review bool) (string, bool) {
	if icon == "" {
		return "", review
	}
	if ok, err := checkIcon(icon); err == nil {
		return ok, review
	}
	return IconFromEmoji(icon)
}
