package envsync

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Claude Code keeps its plugins in ~/.claude/plugins: installed_plugins.json
// lists them, known_marketplaces.json where they come from, and
// settings.json's enabledPlugins which are on. They are read here, and
// changed by its own CLI (claude plugin …), which knows how.

// ClaudePlugins lists Claude Code's plugins and marketplaces under home.
func ClaudePlugins(home string) proto.ClaudePlugins {
	out := proto.ClaudePlugins{Plugins: []proto.ClaudePlugin{}, Marketplaces: []proto.ClaudeMarketplace{}}
	_, err := exec.LookPath("claude")
	out.CLI = err == nil
	dir := filepath.Join(home, ".claude", "plugins")

	var enabled map[string]bool
	if data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json")); err == nil {
		clean, _ := stripJSONC(data)
		var s struct {
			Enabled map[string]bool `json:"enabledPlugins"`
		}
		if json.Unmarshal(clean, &s) == nil {
			enabled = s.Enabled
		}
	}
	var installed struct {
		Plugins map[string][]struct {
			Scope       string `json:"scope"`
			InstallPath string `json:"installPath"`
			Version     string `json:"version"`
		} `json:"plugins"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "installed_plugins.json")); err == nil {
		_ = json.Unmarshal(data, &installed)
	}
	for _, id := range sortedKeys(installed.Plugins) {
		entries := installed.Plugins[id]
		if len(entries) == 0 {
			continue
		}
		e := entries[0]
		name, market, _ := strings.Cut(id, "@")
		p := proto.ClaudePlugin{ID: id, Name: name, Marketplace: market, Version: e.Version, Scope: e.Scope, Enabled: enabled[id]}
		var manifest struct {
			Description string `json:"description"`
		}
		if data, err := os.ReadFile(filepath.Join(e.InstallPath, ".claude-plugin", "plugin.json")); err == nil && json.Unmarshal(data, &manifest) == nil {
			p.Description = manifest.Description
		}
		out.Plugins = append(out.Plugins, p)
	}
	var markets map[string]struct {
		Source struct {
			Source string `json:"source"`
			Repo   string `json:"repo"`
			URL    string `json:"url"`
			Path   string `json:"path"`
		} `json:"source"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "known_marketplaces.json")); err == nil {
		_ = json.Unmarshal(data, &markets)
	}
	for _, name := range sortedKeys(markets) {
		src := markets[name].Source
		where := src.Repo
		if where == "" {
			where = src.URL
		}
		if where == "" {
			where = src.Path
		}
		out.Marketplaces = append(out.Marketplaces, proto.ClaudeMarketplace{Name: name, Source: where})
	}
	return out
}

// PluginArgs is the command line of Claude Code's CLI that does action to
// arg, refusing actions it doesn't offer.
func PluginArgs(action, arg string) ([]string, error) {
	actions := []string{proto.PluginInstall, proto.PluginUninstall, proto.PluginEnable, proto.PluginDisable, proto.PluginUpdate,
		proto.PluginMarketplaceAdd, proto.PluginMarketplaceRemove, proto.PluginMarketplaceUpdate}
	arg = strings.TrimSpace(arg)
	if !slices.Contains(actions, action) {
		return nil, fmt.Errorf("envsync: no plugin action %q", action)
	}
	if arg == "" && action != proto.PluginMarketplaceUpdate || strings.HasPrefix(arg, "-") {
		return nil, fmt.Errorf("envsync: %q isn't a plugin or a marketplace", arg)
	}
	args := append([]string{"plugin"}, strings.Fields(action)...)
	if arg != "" {
		args = append(args, arg)
	}
	return args, nil
}
