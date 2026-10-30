package accounts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/proto"
)

// How an agent's accounts are used: their order, falling back from one at
// its limit to the next, defaults and the projects an account is kept to;
// in the synced settings (settings/account-prefs.json), by account name,
// so every host of the user's reads them for its own accounts of those
// names. The agent's own account is "".

// Prefs is the preferences, by agent base ID. Safe from any goroutine.
type Prefs struct {
	file string
	mu   sync.Mutex
}

// OpenPrefs is the preferences kept in dir.
func OpenPrefs(dir string) *Prefs {
	return &Prefs{file: filepath.Join(dir, "settings", "account-prefs.json")}
}

// All is every agent's preferences.
func (p *Prefs) All() map[string]proto.AccountPrefs {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]proto.AccountPrefs{}
	if data, err := os.ReadFile(p.file); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

// Of is agent's preferences.
func (p *Prefs) Of(agent string) proto.AccountPrefs { return p.All()[agent] }

// Set keeps agent's preferences.
func (p *Prefs) Set(agent string, pr proto.AccountPrefs) error {
	switch pr.Fallback {
	case proto.FallbackOff, proto.FallbackAuto, proto.FallbackAsk:
	default:
		return fmt.Errorf("accounts: no fallback %q: auto, ask or none", pr.Fallback)
	}
	if pr.ToAgent == agent {
		pr.ToAgent = ""
	}
	all := p.All()
	all[agent] = pr
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.file), 0o700); err != nil {
		return err
	}
	return envsync.WriteFileAtomic(p.file, data, 0o600)
}

// Ordered is agent's accounts (its own as "" first unless placed) in the
// preferences' order, those not named after.
func Ordered(pr proto.AccountPrefs, names []string) []string {
	all := append([]string{""}, names...)
	var out []string
	for _, n := range pr.Order {
		if slices.Contains(all, n) && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, n := range all {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// Allowed says account name may be used in project (a folder): it isn't
// kept to other projects.
func Allowed(pr proto.AccountPrefs, name, project string) bool {
	only := pr.Only[name]
	if len(only) == 0 {
		return true
	}
	return slices.ContainsFunc(only, func(p string) bool { return sameProject(p, project) })
}

// Default is the account new sessions of project use: the project's,
// else the agent's default, if allowed there.
func Default(pr proto.AccountPrefs, project string) string {
	for p, name := range pr.Projects {
		if sameProject(p, project) && Allowed(pr, name, project) {
			return name
		}
	}
	if Allowed(pr, pr.Default, project) {
		return pr.Default
	}
	return ""
}

// Next is the account after current in the order, for project, that
// spent doesn't rule out; false when none is left.
func Next(pr proto.AccountPrefs, names []string, current, project string, spent func(name string) bool) (string, bool) {
	order := Ordered(pr, names)
	i := slices.Index(order, current)
	for k := 1; k < len(order); k++ {
		n := order[(i+k)%len(order)]
		if n != current && Allowed(pr, n, project) && !spent(n) {
			return n, true
		}
	}
	return "", false
}

func sameProject(p, project string) bool {
	p = strings.TrimSuffix(p, string(filepath.Separator))
	return p != "" && (p == project || p == filepath.Base(project))
}
