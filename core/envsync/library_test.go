package envsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

func ids(home string) []string {
	var out []string
	for _, it := range Scan(home).Items {
		out = append(out, it.ID)
	}
	return out
}

// An MCP server added shows as each agent keeps it, and deleting it takes
// it out of the agent's config, nothing else.
func TestAddAndDeleteMCPServers(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"theme":"dark","mcpServers":{"keep":{"command":"x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"claude", "codex", "opencode", "gemini"} {
		p, err := NewMCP(agent, "docs", MCPServer{Command: "npx", Args: []string{"-y", "docs-mcp"}, Env: map[string]string{"LEVEL": "1"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Import(home, p); err != nil {
			t.Fatalf("%s: %v", agent, err)
		}
		if !slices.Contains(ids(home), agent+"/mcp/docs") {
			t.Fatalf("%s: not listed after adding: %q", agent, ids(home))
		}
		if err := Delete(home, agent+"/mcp/docs"); err != nil {
			t.Fatalf("%s: %v", agent, err)
		}
		if slices.Contains(ids(home), agent+"/mcp/docs") {
			t.Fatalf("%s: still listed after deleting", agent)
		}
	}
	data, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	if !strings.Contains(string(data), `"theme": "dark"`) || !strings.Contains(string(data), `"keep"`) {
		t.Fatalf("the rest of ~/.claude.json: %s", data)
	}
	if _, err := NewMCP("claude", "both", MCPServer{Command: "x", URL: "https://x"}); err == nil {
		t.Fatal("made a server with a command and a URL")
	}
}

// A skill added from its files lists, and deleting it removes its folder.
func TestAddAndDeleteASkill(t *testing.T) {
	home := t.TempDir()
	p, err := NewSkill("claude", "review", []proto.EnvFile{
		{Path: "SKILL.md", Mode: 0o644, Data: []byte("---\nname: review\n---\nReview it.")},
		{Path: "scripts/check.sh", Mode: 0o755, Data: []byte("#!/bin/sh\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Import(home, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/review/scripts/check.sh")); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids(home), "claude/skill/review") {
		t.Fatalf("not listed: %q", ids(home))
	}
	if err := Delete(home, "claude/skill/review"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/review")); err == nil {
		t.Fatal("the skill's folder stayed")
	}
	if _, err := NewSkill("claude", "x", []proto.EnvFile{{Path: "README.md"}}); err == nil {
		t.Fatal("made a skill without SKILL.md")
	}
}

// A project's own items list from its folder, and one added goes there,
// not in the home folder.
func TestProjectItems(t *testing.T) {
	dir := t.TempDir()
	for rel, text := range map[string]string{"CLAUDE.md": "# Rules", ".claude/commands/ship.md": "Ship it", "AGENTS.md": "# Agents"} {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, it := range ScanIn(dir, Project).Items {
		got = append(got, it.ID)
	}
	for _, want := range []string{"claude/memory/CLAUDE.md", "claude/command/ship.md", "codex/memory/AGENTS.md"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s in %q", want, got)
		}
	}
	p, err := NewMCPIn("claude", "db", MCPServer{Command: "db-mcp"}, Project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportIn(dir, p, Project); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, ".mcp.json")); err != nil || !strings.Contains(string(data), "db-mcp") {
		t.Fatalf(".mcp.json: %s, %v", data, err)
	}
}

// Permissions and hooks edit as lists; the rest of settings.json stays.
func TestClaudeSettingsRoundTrip(t *testing.T) {
	in := []byte(`{
  "model": "opus",
  "permissions": {"allow": ["Bash(npm test:*)"], "defaultMode": "acceptEdits"},
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}],
            "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "check.sh", "timeout": 5}]}]},
  "statusLine": {"type": "command"}
}`)
	perm, hooks, err := ReadClaudeSettings(in)
	if err != nil || len(perm.Allow) != 1 || perm.DefaultMode != "acceptEdits" || len(hooks) != 2 {
		t.Fatalf("read %+v %+v %v", perm, hooks, err)
	}
	if hooks[0].Event != "PreToolUse" || hooks[0].Matcher != "Bash" || hooks[0].Timeout != 5 {
		t.Fatalf("first hook %+v", hooks[0])
	}
	perm.Deny = append(perm.Deny, "Read(./.env)")
	hooks = append(hooks[:1], ClaudeHook{Event: "SessionStart", Command: "echo hi"})
	out, err := WriteClaudeSettings(in, perm, hooks)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"model": "opus"`, `"statusLine"`, `"Read(./.env)"`, `"SessionStart"`, `"check.sh"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in\n%s", want, s)
		}
	}
	if strings.Contains(s, "say done") {
		t.Errorf("a removed hook stayed:\n%s", s)
	}
	if strings.Index(s, `"model"`) > strings.Index(s, `"statusLine"`) {
		t.Error("the file's order changed")
	}
}

// What NewMCP writes, ParseMCP reads back, in every agent's format.
func TestParseMCPRoundTrip(t *testing.T) {
	want := MCPServer{Command: "npx", Args: []string{"-y", "docs"}, Env: map[string]string{"K": "v"}}
	for _, agent := range []string{"claude", "codex", "opencode", "gemini"} {
		p, err := NewMCP(agent, "docs", want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseMCP(p.Item.ID, p.Value)
		if err != nil || got.Command != "npx" || !slices.Equal(got.Args, want.Args) || got.Env["K"] != "v" {
			t.Errorf("%s: %+v, %v\n%s", agent, got, err, p.Value)
		}
	}
}

// A skill, a command, instructions and an MCP server copy to other agents
// as each keeps them.
func TestCopyToAnotherAgent(t *testing.T) {
	home := t.TempDir()
	for rel, text := range map[string]string{
		".claude/skills/review/SKILL.md": "---\nname: review\n---\nReview.",
		".claude/skills/review/lib/x.sh": "echo",
		".claude/commands/ship.md":       "Ship it.",
		".claude/CLAUDE.md":              "# Rules",
	} {
		p := filepath.Join(home, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mcp, _ := NewMCP("claude", "docs", MCPServer{Command: "docs-mcp"})
	if _, err := Import(home, mcp); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, to, want string }{
		{"claude/skill/review", "shared", ".agents/skills/review/lib/x.sh"},
		{"claude/command/ship.md", "opencode", ".config/opencode/commands/ship.md"},
		{"claude/memory/CLAUDE.md", "codex", ".codex/AGENTS.md"},
		{"claude/mcp/docs", "codex", ".codex/config.toml"},
	} {
		p, err := Export(home, c.id)
		if err != nil {
			t.Fatal(err)
		}
		out, err := CopyTo(p, c.to, User)
		if err != nil {
			t.Fatalf("%s → %s: %v", c.id, c.to, err)
		}
		if _, err := Import(home, out); err != nil {
			t.Fatalf("%s → %s: %v", c.id, c.to, err)
		}
		if _, err := os.Stat(filepath.Join(home, c.want)); err != nil {
			t.Errorf("%s → %s: %v", c.id, c.to, err)
		}
	}
	if p, _ := Export(home, "claude/command/ship.md"); slices.Contains(CopyTargets(p, User), "gemini") {
		t.Error("a Markdown command offered to Gemini, whose commands are TOML")
	}
}

// A repository's skills are found, and one picked is added.
func TestSkillsFromARepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	for rel, text := range map[string]string{
		"skills/pdf/SKILL.md":     "---\nname: pdf\ndescription: Reads PDFs\n---\n",
		"skills/pdf/scripts/x.py": "print()",
		"skills/docx/SKILL.md":    "---\nname: docx\n---\n",
		"README.md":               "skills",
	} {
		p := filepath.Join(repo, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(text), 0o644)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-qm", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%s", out)
		}
	}
	dir := filepath.Join(t.TempDir(), "fetch")
	found, err := FetchSkills("file://"+repo, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[1].Name != "pdf" || found[1].Description != "Reads PDFs" {
		t.Fatalf("found %+v", found)
	}
	p, err := SkillFrom(dir, "skills/pdf", "claude", User)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if _, err := Import(home, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/pdf/scripts/x.py")); err != nil {
		t.Fatal(err)
	}
}

func TestRepoURL(t *testing.T) {
	for raw, want := range map[string][3]string{
		"https://github.com/anthropics/skills":                           {"https://github.com/anthropics/skills", "", ""},
		"https://github.com/anthropics/skills/tree/main/document-skills": {"https://github.com/anthropics/skills", "main", "document-skills"},
		"https://github.com/o/r/blob/dev/a/b/SKILL.md":                   {"https://github.com/o/r", "dev", "a/b"},
		"git@github.com:o/r.git":                                         {"git@github.com:o/r.git", "", ""},
	} {
		c, b, f, err := repoURL(raw)
		if err != nil || [3]string{c, b, f} != want {
			t.Errorf("%s: %q %q %q %v", raw, c, b, f, err)
		}
	}
}
