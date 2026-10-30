// Package accounts keeps an agent's other accounts on a host: each a home
// of the agent's own (CLAUDE_CONFIG_DIR, CODEX_HOME) under
// accounts in enowX's folder, sharing the agent's settings, skills and
// conversations through links but keeping its own sign-in. A team's
// member's account on a host they visit shares nothing.
package accounts

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

	"github.com/enowx/enowx/proto"
)

// kind is how an agent takes another home.
type kind struct {
	env  string   // the variable naming the home
	home string   // the default home, under the user's
	own  []string // what in it is the account's, not shared
}

// kinds are the agents that can have accounts, by base ID.
var kinds = map[string]kind{
	// .claude.json holds who is signed in; its backups would offer the
	// default account's back.
	"claude": {env: "CLAUDE_CONFIG_DIR", home: ".claude", own: []string{".credentials.json", ".claude.json", ".claude.json.lock", "backups"}},
	"codex":  {env: "CODEX_HOME", home: ".codex", own: []string{"auth.json"}},
}

// Supported reports whether agent (a base ID) can have accounts.
func Supported(agent string) bool {
	_, ok := kinds[agent]
	return ok
}

// Store is a host's accounts, in dir/accounts.json, each home in
// dir/accounts/<id>. Safe from any goroutine.
type Store struct {
	dir string

	mu   sync.Mutex
	list []proto.Account
}

// Open loads the accounts kept in dir.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir}
	data, err := os.ReadFile(s.file())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("accounts: %w", err)
	}
	if err := json.Unmarshal(data, &s.list); err != nil {
		return nil, fmt.Errorf("accounts: read %s: %w", s.file(), err)
	}
	return s, nil
}

func (s *Store) file() string { return filepath.Join(s.dir, "accounts.json") }

// Home is account id's home.
func (s *Store) Home(id string) string { return filepath.Join(s.dir, "accounts", id) }

// List returns the accounts in the order they were added.
func (s *Store) List() []proto.Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.list)
}

// Get returns account id.
func (s *Store) Get(id string) (proto.Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.list, func(a proto.Account) bool { return a.ID == id })
	if i < 0 {
		return proto.Account{}, false
	}
	return s.list[i], true
}

// Add keeps a new account of agent, named name, and makes its home.
func (s *Store) Add(agent, name string) (proto.Account, error) {
	name = strings.TrimSpace(name)
	if !Supported(agent) {
		return proto.Account{}, fmt.Errorf("accounts: %s can't have other accounts", agent)
	}
	if name == "" {
		return proto.Account{}, errors.New("accounts: name the account")
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	acc := proto.Account{ID: hex.EncodeToString(b), Agent: agent, Name: name}
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.ContainsFunc(s.list, func(a proto.Account) bool { return a.Agent == agent && strings.EqualFold(a.Name, name) }) {
		return proto.Account{}, fmt.Errorf("accounts: %s already has an account named %s", agent, name)
	}
	if err := os.MkdirAll(s.Home(acc.ID), 0o700); err != nil {
		return proto.Account{}, fmt.Errorf("accounts: %w", err)
	}
	s.list = append(s.list, acc)
	if err := s.save(); err != nil {
		s.list = s.list[:len(s.list)-1]
		return proto.Account{}, err
	}
	return acc, nil
}

// AddKey keeps a new account of agent that is an API key: the vault's
// credential cred, lent to its sessions, with no home of its own.
func (s *Store) AddKey(agent, name, cred string) (proto.Account, error) {
	name, cred = strings.TrimSpace(name), strings.TrimSpace(cred)
	if name == "" || cred == "" {
		return proto.Account{}, errors.New("accounts: name the account and its credential")
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	acc := proto.Account{ID: hex.EncodeToString(b), Agent: agent, Name: name, Cred: cred}
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.ContainsFunc(s.list, func(a proto.Account) bool { return a.Agent == agent && strings.EqualFold(a.Name, name) }) {
		return proto.Account{}, fmt.Errorf("accounts: %s already has an account named %s", agent, name)
	}
	s.list = append(s.list, acc)
	if err := s.save(); err != nil {
		s.list = s.list[:len(s.list)-1]
		return proto.Account{}, err
	}
	return acc, nil
}

// ByName is agent's account named name ("" its own, false).
func (s *Store) ByName(agent, name string) (proto.Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.list, func(a proto.Account) bool {
		return a.Agent == agent && a.Member == "" && strings.EqualFold(a.Name, name)
	})
	if name == "" || i < 0 {
		return proto.Account{}, false
	}
	return s.list[i], true
}

// ForMember is agent's account for member, a team's member's cloud
// account, if the host has made one.
func (s *Store) ForMember(agent, member string) (proto.Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.list, func(a proto.Account) bool { return member != "" && a.Agent == agent && a.Member == member })
	if i < 0 {
		return proto.Account{}, false
	}
	return s.list[i], true
}

// AddMember keeps a new account of agent for member, a team's member's
// cloud account, labelled name: isolated, its home links nothing of the
// host's user's (no settings, skills or conversations). Its label may
// repeat another's; member may have one account of agent.
func (s *Store) AddMember(agent, name, member string) (proto.Account, error) {
	name = strings.TrimSpace(name)
	if !Supported(agent) {
		return proto.Account{}, fmt.Errorf("accounts: %s can't have other accounts", agent)
	}
	if name == "" || member == "" {
		return proto.Account{}, errors.New("accounts: a member's account needs a label and the member")
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	acc := proto.Account{ID: hex.EncodeToString(b), Agent: agent, Name: name, Member: member}
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.ContainsFunc(s.list, func(a proto.Account) bool { return a.Agent == agent && a.Member == member }) {
		return proto.Account{}, fmt.Errorf("accounts: %s already has an account for that member", agent)
	}
	if err := os.MkdirAll(s.Home(acc.ID), 0o700); err != nil {
		return proto.Account{}, fmt.Errorf("accounts: %w", err)
	}
	s.list = append(s.list, acc)
	if err := s.save(); err != nil {
		s.list = s.list[:len(s.list)-1]
		return proto.Account{}, err
	}
	return acc, nil
}

// Remove forgets account id and deletes its home, its sign-in with it.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.list, func(a proto.Account) bool { return a.ID == id })
	if i < 0 {
		return fmt.Errorf("accounts: no account %s", id)
	}
	s.list = slices.Delete(s.list, i, i+1)
	if err := s.save(); err != nil {
		return err
	}
	return os.RemoveAll(s.Home(id))
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	tmp := s.file() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("accounts: %w", err)
	}
	return os.Rename(tmp, s.file())
}

// Env is what a session of agent (a base ID) signed in as account id
// runs with, its home linked to the agent's own first; nil for the
// default account.
func (s *Store) Env(agent, id string) ([]string, error) {
	if id == "" {
		return nil, nil
	}
	acc, ok := s.Get(id)
	if !ok {
		return nil, fmt.Errorf("accounts: no account %s", id)
	}
	if acc.Agent != agent {
		return nil, fmt.Errorf("accounts: %s is an account of %s, not %s", acc.Name, acc.Agent, agent)
	}
	if acc.Cred != "" {
		return nil, nil // a key: the vault lends it, the agent's own home
	}
	k := kinds[agent]
	home := s.Home(id)
	link := share
	if acc.Member != "" {
		link = isolate
	}
	if err := link(k, home); err != nil {
		return nil, err
	}
	return []string{k.env + "=" + home}, nil
}

// isolate makes home, a member's account's, and takes any link out of it:
// nothing in it may lead to the host's user's files.
func isolate(_ kind, home string) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(filepath.Join(home, e.Name())); err != nil {
				return fmt.Errorf("accounts: %w", err)
			}
		}
	}
	return nil
}

// share links what is in the agent's default home into home, but for the
// account's own files, so settings, skills and conversations are the same
// whichever account is used. What home has already stays.
func share(k kind, home string) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	src := filepath.Join(userHome, k.home)
	entries, err := os.ReadDir(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if slices.Contains(k.own, e.Name()) {
			continue
		}
		link := filepath.Join(home, e.Name())
		if _, err := os.Lstat(link); err == nil {
			continue
		}
		// Where links can't be made (Windows without the right), the
		// account works without what it would share.
		_ = os.Symlink(filepath.Join(src, e.Name()), link)
	}
	return nil
}
