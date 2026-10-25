package usersync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

func encodeConflict(c proto.SyncConflict) ([]byte, error) { return json.MarshalIndent(c, "", "  ") }

func (f *Folder) conflictFile(id string) string {
	return filepath.Join(f.dir, filepath.FromSlash(ConflictsDir), filepath.Base(id)+".json")
}

// Conflicts lists the conflicts waiting for the user, the oldest first.
func (f *Folder) Conflicts() ([]proto.SyncConflict, error) {
	dir := filepath.Join(f.dir, filepath.FromSlash(ConflictsDir))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []proto.SyncConflict{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("usersync: %w", err)
	}
	out := []proto.SyncConflict{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var c proto.SyncConflict
		if json.Unmarshal(data, &c) == nil && c.ID != "" {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b proto.SyncConflict) int { return a.Created.Compare(b.Created) })
	return out, nil
}

// Resolve settles conflict id: the version of host keep takes the file's
// place, or with keep "both", the one in place stays and each other
// version is kept beside it, a note as a copy of its own.
func (f *Folder) Resolve(id, keep string) error {
	data, err := os.ReadFile(f.conflictFile(id))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("usersync: no conflict %q", id)
	}
	if err != nil {
		return fmt.Errorf("usersync: %w", err)
	}
	var c proto.SyncConflict
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("usersync: conflict %s: %w", id, err)
	}
	if !Synced(c.Path) {
		return fmt.Errorf("usersync: conflict %s names %q", id, c.Path)
	}
	cur, err := f.Read([]string{c.Path})
	if err != nil {
		return err
	}
	expect := ""
	if d, ok := cur[c.Path]; ok {
		expect = Hash(d)
	}
	var changes []proto.SyncChange
	if keep == "both" {
		for i, v := range c.Versions {
			if v.Deleted || expect != "" && Hash(v.Data) == expect {
				continue
			}
			p, data := copyPath(c.Path, i), v.Data
			if isNote(c.Path) {
				data = renameNote(data, v.Host)
			}
			changes = append(changes, proto.SyncChange{Path: p, Data: data})
		}
	} else {
		i := slices.IndexFunc(c.Versions, func(v proto.SyncVersion) bool { return v.Host == keep })
		if i < 0 {
			var hosts []string
			for _, v := range c.Versions {
				hosts = append(hosts, v.Host)
			}
			return fmt.Errorf("usersync: conflict %s has versions of %s, or keep both", id, strings.Join(hosts, ", "))
		}
		v := c.Versions[i]
		changes = append(changes, proto.SyncChange{Path: c.Path, Data: v.Data, Delete: v.Deleted, Expect: expect})
	}
	changes = append(changes, proto.SyncChange{Path: ConflictsDir + c.ID + ".json", Delete: true, Expect: Hash(data)})
	_, stale, err := f.Write(changes)
	if err != nil {
		return err
	}
	if len(stale) > 0 {
		return fmt.Errorf("usersync: %s changed meanwhile; try again", strings.Join(stale, ", "))
	}
	return nil
}

// copyPath is where version i of p goes beside it: a note gets an ID of
// its own.
func copyPath(p string, i int) string {
	if isNote(p) {
		var b [6]byte
		rand.Read(b[:])
		return "notes/" + hex.EncodeToString(b[:]) + ".md"
	}
	ext := path.Ext(p)
	return fmt.Sprintf("%s.conflict-%d%s", strings.TrimSuffix(p, ext), i+1, ext)
}

// renameNote titles a note's copy after the host it came from.
func renameNote(data []byte, host string) []byte {
	head, body, ok := splitNote(string(data))
	if !ok {
		return data
	}
	var b strings.Builder
	b.WriteString("---\n")
	for _, kv := range head {
		if kv[0] == "title" {
			kv[1] += " (" + host + ")"
		}
		b.WriteString(kv[0] + ": " + kv[1] + "\n")
	}
	b.WriteString("---\n" + body)
	return []byte(b.String())
}
