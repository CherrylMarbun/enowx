// Package projectlinks remembers the copies of a project on several
// hosts: which is the source and which the copy, how they sync on their
// own, and how a project's work is pushed on its own. What the user sets
// is kept with their data, synced to every host (settings/*.json under a
// daemon's folder); how each sync and push went stays on the host.
package projectlinks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LocalDir is a project's folder of enowX's own that is never shared:
// it ignores itself in git, and syncs and copies leave it out.
const LocalDir = ".enowx/local"

// identity is a project's .enowx/local/project.json.
type identity struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

func identityFile(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(LocalDir), "project.json")
}

// ReadID is the ID of the project at dir, "" when it has none.
func ReadID(dir string) string {
	data, err := os.ReadFile(identityFile(dir))
	if err != nil {
		return ""
	}
	var id identity
	if json.Unmarshal(data, &id) != nil || !ValidID(id.ID) {
		return ""
	}
	return id.ID
}

// ValidID reports whether id is a project's ID.
func ValidID(id string) bool {
	if len(id) != 16 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// EnsureID is the ID of the project at dir, made when it has none.
func EnsureID(dir string) (string, error) {
	if id := ReadID(dir); id != "" {
		return id, nil
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	return id, WriteID(dir, id)
}

// WriteID gives the project at dir the ID id: a copy takes its source's.
func WriteID(dir, id string) error {
	if !ValidID(id) {
		return errors.New("projectlinks: not a project's ID: " + id)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("projectlinks: %s isn't a folder", dir)
	}
	local := filepath.Join(dir, filepath.FromSlash(LocalDir))
	if err := os.MkdirAll(local, 0o755); err != nil {
		return fmt.Errorf("projectlinks: %w", err)
	}
	// Git leaves the folder out, whatever the project's .gitignore says.
	if err := os.WriteFile(filepath.Join(local, ".gitignore"), []byte("*\n"), 0o644); err != nil {
		return fmt.Errorf("projectlinks: %w", err)
	}
	data, _ := json.MarshalIndent(identity{ID: id, Created: time.Now().UTC().Truncate(time.Second)}, "", "  ")
	return writeFile(identityFile(dir), data)
}

// writeFile replaces the file at path, whole or not at all.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("projectlinks: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("projectlinks: %w", err)
	}
	return os.Rename(tmp, path)
}
