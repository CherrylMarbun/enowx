// Package hostauth keeps a host's sign-in to a cloud: the cloud's address,
// the device's key pair and its token, in files only the host's user reads.
// The private key and the token never leave the host.
package hostauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/enowx/enowx/cloud/seal"
)

// State is what is kept of the sign-in.
type State struct {
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
	// Who the token is, as the cloud said when signing in.
	AccountID  string `json:"accountId,omitempty"`
	Account    string `json:"account,omitempty"`
	GitHub     string `json:"github,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	DeviceID   string `json:"deviceId,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	DeviceKind string `json:"deviceKind,omitempty"`
}

// SignedIn says the state has a token.
func (s State) SignedIn() bool { return s.URL != "" && s.Token != "" }

// Store is the sign-in kept in a folder (the host's enowX folder's cloud/).
// It is safe for use from several goroutines.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open is the store in dir.
func Open(dir string) *Store { return &Store{dir: dir} }

func (s *Store) file() string { return filepath.Join(s.dir, "auth.json") }

// Get is the sign-in kept, empty when none.
func (s *Store) Get() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st State
	data, err := os.ReadFile(s.file())
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("hostauth: %w", err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("hostauth: %s: %w", s.file(), err)
	}
	return st, nil
}

// Put keeps st.
func (s *Store) Put(st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(s.dir, s.file(), data)
}

// DeviceKey is the host's key pair, made the first time it is asked for.
func (s *Store) DeviceKey() (*seal.DeviceKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file := filepath.Join(s.dir, "device.key")
	data, err := os.ReadFile(file)
	if err == nil {
		return seal.LoadDeviceKey(data)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("hostauth: %w", err)
	}
	k, err := seal.NewDeviceKey()
	if err != nil {
		return nil, err
	}
	return k, writePrivate(s.dir, file, k.Private())
}

// writePrivate writes data to file, readable by the host's user only,
// whole or not at all.
func writePrivate(dir, file string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("hostauth: %w", err)
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("hostauth: %w", err)
	}
	if err := os.Rename(tmp, file); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("hostauth: %w", err)
	}
	return nil
}
