// Package hosts keeps the servers a device connects to. The list lives on
// the device (not on any server), in one JSON file the user can copy.
package hosts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// LocalID names this machine, which is always in the list and never stored.
const LocalID = "local"

// Host is a server reachable over SSH.
type Host struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`                // user@host[:port]; host may be a Tailscale name
	IdentityFile string `json:"identityFile,omitempty"` // private key to try first
	// PasswordAuth says the user logs in with a password, which the app
	// keeps in the system's credential store, never in this file.
	PasswordAuth bool `json:"passwordAuth,omitempty"`
}

// PasswordKey is the credential store key of a host's SSH password.
func PasswordKey(id string) string { return "ssh-password:" + id }

// Store is the list of hosts, saved as JSON in one file.
type Store struct {
	path string

	mu    sync.Mutex
	hosts []Host
}

// Open loads the store at path, which may not exist yet.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("hosts: %w", err)
	}
	if err := json.Unmarshal(data, &s.hosts); err != nil {
		return nil, fmt.Errorf("hosts: read %s: %w", path, err)
	}
	return s, nil
}

// List returns the hosts in the order they were added.
func (s *Store) List() []Host {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.hosts)
}

// Get returns the host with the given ID.
func (s *Store) Get(id string) (Host, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.hosts, func(h Host) bool { return h.ID == id })
	if i < 0 {
		return Host{}, false
	}
	return s.hosts[i], true
}

// Add stores h with a new ID and returns it.
func (s *Store) Add(h Host) (Host, error) {
	h.Name, h.Address = strings.TrimSpace(h.Name), strings.TrimSpace(h.Address)
	if h.Address == "" {
		return Host{}, errors.New("hosts: the address is empty")
	}
	if h.Name == "" {
		h.Name = h.Address
	}
	h.ID = newID()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts = append(s.hosts, h)
	if err := s.save(); err != nil {
		s.hosts = s.hosts[:len(s.hosts)-1]
		return Host{}, err
	}
	return h, nil
}

// Remove forgets a host. Its server and sessions are left untouched.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.hosts, func(h Host) bool { return h.ID == id })
	if i < 0 {
		return fmt.Errorf("hosts: no host %s", id)
	}
	s.hosts = slices.Delete(s.hosts, i, i+1)
	return s.save()
}

// save writes the list atomically; s.mu is held.
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.hosts, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
