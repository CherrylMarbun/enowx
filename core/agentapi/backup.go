package agentapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Before enowX changes an agent's files, it keeps them as they were, in
// ~/.enowx-app/agentapi/<agent>/<when>/: the first of these, the files as
// the user had them, for good; the others, before each change after, the
// last few. Restoring one puts its files back as they were, removing
// those that weren't there.

// keepBackups is how many backups after the first an agent keeps.
const keepBackups = 10

// Backup is a backup of an agent's files.
type Backup struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"time"`
	Original bool      `json:"original,omitempty"` // the files as the user had them
	Note     string    `json:"note,omitempty"`     // what change came after
	Files    []string  `json:"files"`              // under the home
}

// backedFile is a file in a backup: its content, or that it wasn't there.
type backedFile struct {
	Path   string `json:"path"` // under the home
	Absent bool   `json:"absent,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
}

type manifest struct {
	Backup
	Entries []backedFile `json:"entries"`
}

func backupsDir(home, agent string) string {
	return filepath.Join(home, ".enowx-app", "agentapi", agent)
}

// Backups are agent's backups, newest first, the original last.
func Backups(home, agent string) ([]Backup, error) {
	entries, err := os.ReadDir(backupsDir(home, agent))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Backup
	for _, e := range entries {
		m, err := readManifest(home, agent, e.Name())
		if err != nil {
			continue
		}
		out = append(out, m.Backup)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Original != out[j].Original {
			return out[j].Original
		}
		return out[i].Time.After(out[j].Time)
	})
	return out, nil
}

func readManifest(home, agent, id string) (manifest, error) {
	var m manifest
	data, err := os.ReadFile(filepath.Join(backupsDir(home, agent), id, "manifest.json"))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(data, &m)
}

// backup keeps files (under home) of agent before a change, note saying
// which; the first is the original.
func backup(home, agent string, files []string, note string) (Backup, error) {
	existing, err := Backups(home, agent)
	if err != nil {
		return Backup{}, err
	}
	now := time.Now()
	id := now.Format("20060102-150405.000000")
	b := Backup{ID: id, Time: now, Note: note, Files: files, Original: !slices.ContainsFunc(existing, func(b Backup) bool { return b.Original })}
	dir := filepath.Join(backupsDir(home, agent), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return b, err
	}
	m := manifest{Backup: b}
	for i, rel := range files {
		data, err := os.ReadFile(filepath.Join(home, rel))
		if errors.Is(err, fs.ErrNotExist) {
			m.Entries = append(m.Entries, backedFile{Path: rel, Absent: true})
			continue
		}
		if err != nil {
			return b, err
		}
		mode := uint32(0o600)
		if fi, err := os.Stat(filepath.Join(home, rel)); err == nil {
			mode = uint32(fi.Mode().Perm())
		}
		// The files hold API keys: only the user reads them.
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprint(i)), data, 0o600); err != nil {
			return b, err
		}
		m.Entries = append(m.Entries, backedFile{Path: rel, Mode: mode})
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
		return b, err
	}
	// The original stays; of the others, the newest few.
	var others []Backup
	for _, x := range existing {
		if !x.Original {
			others = append(others, x)
		}
	}
	for i, x := range others {
		if i >= keepBackups-1 {
			os.RemoveAll(filepath.Join(backupsDir(home, agent), x.ID))
		}
	}
	return b, nil
}

// Restore puts agent's files back as backup id kept them. The files as
// they are now are kept first, so a restore can be undone too.
func Restore(home, agent, id string) error {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return errors.New("agentapi: which backup?")
	}
	m, err := readManifest(home, agent, id)
	if err != nil {
		return fmt.Errorf("agentapi: no backup %s of %s", id, agent)
	}
	var files []string
	for _, e := range m.Entries {
		files = append(files, e.Path)
	}
	if _, err := backup(home, agent, files, "restoring "+m.Time.Format("2006-01-02 15:04")); err != nil {
		return err
	}
	dir := filepath.Join(backupsDir(home, agent), id)
	for i, e := range m.Entries {
		path := filepath.Join(home, e.Path)
		if e.Absent {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, fmt.Sprint(i)))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, fs.FileMode(e.Mode)); err != nil {
			return err
		}
	}
	return nil
}
