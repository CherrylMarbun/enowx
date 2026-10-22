package envsync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

func write(t *testing.T, home, rel, data string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(home, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, home, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func items(scan proto.EnvScan) map[string]proto.EnvItem {
	m := map[string]proto.EnvItem{}
	for _, it := range scan.Items {
		if it.Kind != proto.EnvGit { // the machine's own git config
			m[it.ID] = it
		}
	}
	return m
}

// sync copies every item from one home to the other, as the app does.
func sync(t *testing.T, from, to string) []string {
	t.Helper()
	var warnings []string
	for id := range items(Scan(from)) {
		p, err := Export(from, id)
		if err != nil {
			t.Fatalf("export %s: %v", id, err)
		}
		// The payload crosses the wire as JSON.
		raw, _ := json.Marshal(p)
		var got proto.EnvPayload
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		res, err := Import(to, got)
		if err != nil {
			t.Fatalf("import %s: %v", id, err)
		}
		warnings = append(warnings, res.Warnings...)
	}
	return warnings
}

func TestSyncCopiesTheEnvironment(t *testing.T) {
	mac, vps := t.TempDir(), t.TempDir()

	write(t, mac, ".claude/skills/review/SKILL.md", "---\nname: review\n---\nRun "+mac+"/bin/lint first.\n", 0o644)
	write(t, mac, ".claude/skills/review/scripts/check.sh", "#!/bin/sh\necho ok\n", 0o755)
	write(t, mac, ".claude/skills/not-a-skill/notes.md", "no SKILL.md here", 0o644)
	write(t, mac, ".claude/commands/ship.md", "Ship it.", 0o644)
	write(t, mac, ".claude/CLAUDE.md", "Be brief.", 0o644)
	write(t, mac, ".claude.json", `{"numStartups": 3, "mcpServers": {
		"github": {"command": "npx", "args": ["-y", "server-github"], "env": {"GITHUB_TOKEN": "ghp_x"}},
		"files": {"command": "`+mac+`/bin/files-mcp", "args": []}
	}}`, 0o600)
	write(t, mac, ".codex/config.toml", `model = "gpt-5"

# Docs search
[mcp_servers.docs]
command = "docs-mcp"
args = ["--root", "`+mac+`/docs"]

[mcp_servers.docs.env]
DOCS_KEY = "k"
`, 0o600)
	write(t, mac, ".config/opencode/opencode.jsonc", `{
  // my servers
  "mcp": {
    "jira": {"type": "remote", "url": "https://jira.example.com/mcp?api_key=1", "enabled": true,},
  },
}`, 0o600)

	// The server has its own state, which must survive.
	write(t, vps, ".claude.json", `{"userID": "abc", "mcpServers": {"local-only": {"command": "x"}}}`, 0o600)
	write(t, vps, ".codex/config.toml", `# server config
approval_policy = "never"

[mcp_servers.other]
command = "other"
`, 0o600)

	scan := items(Scan(mac))
	want := []string{
		"claude/skill/review", "claude/command/ship.md", "claude/memory/CLAUDE.md",
		"claude/mcp/github", "claude/mcp/files", "codex/mcp/docs", "opencode/mcp/jira",
	}
	for _, id := range want {
		if _, ok := scan[id]; !ok {
			t.Errorf("scan misses %s; has %v", id, slices.Sorted(func(yield func(string) bool) {
				for k := range scan {
					if !yield(k) {
						return
					}
				}
			}))
		}
	}
	if _, ok := scan["claude/skill/not-a-skill"]; ok {
		t.Error("a folder without SKILL.md counted as a skill")
	}
	if got := scan["claude/mcp/github"].Secrets; !slices.Equal(got, []string{"env GITHUB_TOKEN"}) {
		t.Errorf("github secrets = %v", got)
	}
	if got := scan["codex/mcp/docs"].Secrets; !slices.Equal(got, []string{"env DOCS_KEY"}) {
		t.Errorf("docs secrets = %v", got)
	}
	if got := scan["opencode/mcp/jira"].Secrets; !slices.Equal(got, []string{"url parameter api_key"}) {
		t.Errorf("jira secrets = %v", got)
	}

	warnings := sync(t, mac, vps)

	// Every item now matches, though the homes differ.
	after := items(Scan(vps))
	for id, it := range scan {
		if after[id].Hash != it.Hash {
			t.Errorf("%s differs after sync", id)
		}
	}
	if !strings.Contains(read(t, vps, ".claude/skills/review/SKILL.md"), vps+"/bin/lint") {
		t.Error("home path in a skill was not moved")
	}
	if info, err := os.Stat(filepath.Join(vps, ".claude/skills/review/scripts/check.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("script lost its exec bit: %v", err)
	}
	claude := read(t, vps, ".claude.json")
	for _, s := range []string{`"userID": "abc"`, `"local-only"`, `"github"`, vps + `/bin/files-mcp`} {
		if !strings.Contains(claude, s) {
			t.Errorf(".claude.json lacks %s:\n%s", s, claude)
		}
	}
	if strings.Index(claude, "userID") > strings.Index(claude, "mcpServers") {
		t.Error(".claude.json keys were reordered")
	}
	codex := read(t, vps, ".codex/config.toml")
	for _, s := range []string{"# server config", "[mcp_servers.other]", "[mcp_servers.docs.env]", vps + "/docs"} {
		if !strings.Contains(codex, s) {
			t.Errorf("config.toml lacks %s:\n%s", s, codex)
		}
	}
	if read(t, vps, ".claude.json.enowx.bak") == "" {
		t.Error("no backup of .claude.json")
	}
	if !slices.ContainsFunc(warnings, func(w string) bool { return strings.Contains(w, "docs-mcp") }) {
		t.Errorf("no warning about the missing docs-mcp program: %v", warnings)
	}

	// Syncing again changes nothing.
	sync(t, mac, vps)
	if read(t, vps, ".codex/config.toml") != codex {
		t.Error("a second sync changed config.toml")
	}
}

func TestImportRejectsPathsOutsideTheItem(t *testing.T) {
	home := t.TempDir()
	evil := proto.EnvPayload{
		Item:  proto.EnvItem{ID: "claude/skill/x"},
		Files: []proto.EnvFile{{Path: ".claude/skills/x/../../../.bashrc", Mode: 0o644, Data: []byte("pwned")}},
	}
	if _, err := Import(home, evil); err == nil {
		t.Fatal("imported a file outside the skill")
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
		t.Fatal(".bashrc was written")
	}
	for _, id := range []string{"claude/skill/..", "claude/skill/a/b", "nope/skill/x"} {
		if _, err := Import(home, proto.EnvPayload{Item: proto.EnvItem{ID: id}}); err == nil {
			t.Errorf("accepted item %q", id)
		}
	}
}

func TestRewriteHomeMovesWholePathsOnly(t *testing.T) {
	got := rewriteHome("/Users/me /Users/me/x /Users/meg \"/Users/me\"", "/Users/me", "/home/enowx")
	if want := "/home/enowx /home/enowx/x /Users/meg \"/home/enowx\""; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
