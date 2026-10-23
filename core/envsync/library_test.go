package envsync

import (
	"os"
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
