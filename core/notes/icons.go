package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/enowx/enowx/core/iconlib"
	"github.com/enowx/enowx/proto"
)

// Icons of notes and groups are SVG (core/iconlib): "lucide:rocket", or
// the user's own. Emoji, their icons of before, move to the closest SVG
// once (MigrateIcons); what has none close gets the default, marked for
// the user to pick again. Emoji in a note's text stay: only icons changed.

// IconFromEmoji is the SVG icon for emoji e, as notes brought from
// elsewhere (an import) get: the closest, else the default, review true.
func IconFromEmoji(e string) (icon string, review bool) {
	ref, known := iconlib.FromEmoji(e)
	return ref, !known
}

// checkIcon is icon as stored, or why it isn't one.
func checkIcon(icon string) (string, error) {
	out, err := iconlib.Check(icon)
	if err != nil {
		return "", fmt.Errorf("notes: %w", err)
	}
	return out, nil
}

// MigrateIcons moves the emoji icons of notes and groups to SVG, and a
// title's leading emoji into its note's icon, and says how many changed.
func (s *Store) MigrateIcons() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.List()
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, info := range list {
		emoji, rest := iconlib.LeadingEmoji(info.Title)
		if !iconlib.IsEmoji(info.Icon) && emoji == "" {
			continue
		}
		n, err := s.read(info.ID, true)
		if err != nil {
			return changed, err
		}
		if emoji != "" {
			n.Title = rest
			if n.Icon == "" {
				n.Icon = emoji
			}
		}
		if iconlib.IsEmoji(n.Icon) {
			n.Icon, n.IconReview = IconFromEmoji(n.Icon)
		}
		if err := s.write(n); err != nil {
			return changed, err
		}
		changed++
	}
	groups, err := s.Groups()
	if err != nil {
		return changed, err
	}
	moved := false
	for i, g := range groups {
		if iconlib.IsEmoji(g.Icon) {
			groups[i].Icon, groups[i].IconReview = IconFromEmoji(g.Icon)
			moved = true
			changed++
		}
	}
	if moved {
		return changed, s.saveGroups(groups)
	}
	return changed, nil
}

// Icon preferences, in icons.json beside the notes: the icons used
// lately, the user's favorites, and the libraries the picker shows.

const maxRecentIcons = 24

func (s *Store) iconPrefsFile() string { return filepath.Join(s.dir, "icons.json") }

// IconPrefs is the icon preferences.
func (s *Store) IconPrefs() (proto.NoteIconPrefs, error) {
	p := proto.NoteIconPrefs{Recent: []string{}, Favorites: []string{}, Libraries: []string{}}
	data, err := os.ReadFile(s.iconPrefsFile())
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, fmt.Errorf("notes: %w", err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("notes: icons: %w", err)
	}
	return p, nil
}

// SaveIconPrefs keeps the favorites and libraries of p (the recent icons
// follow use) and answers the preferences.
func (s *Store) SaveIconPrefs(p proto.NoteIconPrefs) (proto.NoteIconPrefs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.IconPrefs()
	if err != nil {
		return cur, err
	}
	var favs []string
	for _, f := range p.Favorites {
		if ref, err := iconlib.Check(f); err == nil && ref != "" && !slices.Contains(favs, ref) {
			favs = append(favs, ref)
		}
	}
	var libs []string
	for _, l := range p.Libraries {
		if slices.ContainsFunc(iconlib.Libraries, func(x iconlib.Library) bool { return x.ID == l }) && !slices.Contains(libs, l) {
			libs = append(libs, l)
		}
	}
	cur.Favorites, cur.Libraries = orEmpty(favs), orEmpty(libs)
	return cur, s.writeIconPrefs(cur)
}

// usedIcon puts icon first in the recent icons; s.mu is held.
func (s *Store) usedIcon(icon string) error {
	if icon == "" {
		return nil
	}
	p, err := s.IconPrefs()
	if err != nil {
		return err
	}
	p.Recent = slices.DeleteFunc(p.Recent, func(r string) bool { return r == icon })
	p.Recent = slices.Insert(p.Recent, 0, icon)
	if len(p.Recent) > maxRecentIcons {
		p.Recent = p.Recent[:maxRecentIcons]
	}
	return s.writeIconPrefs(p)
}

func (s *Store) writeIconPrefs(p proto.NoteIconPrefs) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	tmp := s.iconPrefsFile() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	return os.Rename(tmp, s.iconPrefsFile())
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
