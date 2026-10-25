package agentapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The user's own settings, which must come through every change.
var userFiles = map[string]string{
	".claude/settings.json":      "{\n  // mine\n  \"theme\": \"dark\",\n  \"env\": {\"MY_VAR\": \"1\"}\n}\n",
	".codex/config.toml":         "approval_policy = \"never\"\n\n[mcp_servers.docs]\ncommand = \"docs-mcp\"\n",
	".config/opencode/opencode.json": "{\"theme\": \"opencode\", \"provider\": {\"mine\": {\"options\": {}}}}\n",
	".gemini/settings.json":      "{\"ui\": {\"theme\": \"GitHub\"}}\n",
	".gemini/.env":               "OTHER=1\n",
	".qwen/settings.json":        "{\"modelProviders\": {\"openai\": [{\"id\": \"theirs\", \"envKey\": \"X\"}]}}\n",
	".factory/settings.json":     "{\"customModels\": [{\"model\": \"theirs\", \"displayName\": \"Theirs\"}]}\n",
	".config/goose/config.yaml":  "GOOSE_MODE: auto\nextensions:\n  dev:\n    enabled: true\n",
	".aider.conf.yml":            "dark-mode: true\n",
	".kimi/config.toml":          "theme = \"dark\"\n\n[providers.kimi]\ntype = \"kimi\"\n",
	".vibe/config.toml":          "theme = \"x\"\n\n[[providers]]\nname = \"mistral\"\napi_base = \"https://api.mistral.ai/v1\"\n",
	".pi/agent/settings.json":    "{\"theme\": \"light\"}\n",
}

func TestEveryAgentTakesAnAPIAndGivesItBack(t *testing.T) {
	home := t.TempDir()
	for rel, text := range userFiles {
		os.MkdirAll(filepath.Join(home, filepath.Dir(rel)), 0o755)
		os.WriteFile(filepath.Join(home, rel), []byte(text), 0o644)
	}
	for _, id := range Supported() {
		a := adapters[id]
		for _, proto := range a.protocols {
			api := API{BaseURL: "https://gw.example.com/v1", APIKey: "sk-test-" + id, Model: "gw-model", Protocol: proto}
			if a.small {
				api.SmallModel = "gw-small"
			}
			if a.noModel {
				api.Model = ""
			}
			if err := Set(home, id, api); err != nil {
				t.Fatalf("%s %s: set: %v", id, proto, err)
			}
			got, err := Of(home, id)
			if err != nil || got.Current == nil {
				t.Fatalf("%s %s: read back: %+v %v", id, proto, got, err)
			}
			if c := *got.Current; c != api {
				t.Errorf("%s %s: read back %+v, want %+v", id, proto, c, api)
			}
		}
		// The user's own settings stayed.
		for rel, text := range userFiles {
			if !slicesContains(a.files, rel) {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(home, rel))
			for _, keep := range []string{"dark", "docs-mcp", "opencode", "GitHub", "OTHER", "theirs", "GOOSE_MODE", "dark-mode", "mistral", "light", "MY_VAR"} {
				if strings.Contains(text, keep) && !strings.Contains(string(data), keep) {
					t.Errorf("%s: %s lost %q:\n%s", id, rel, keep, data)
				}
			}
		}
		if err := Clear(home, id); err != nil {
			t.Fatalf("%s: clear: %v", id, err)
		}
		if got, _ := Of(home, id); got.Current != nil {
			t.Errorf("%s: still set after clear: %+v", id, got.Current)
		}
		// The original backup puts the files back byte for byte.
		bs, _ := Backups(home, id)
		orig := ""
		for _, b := range bs {
			if b.Original {
				orig = b.ID
			}
		}
		if orig == "" {
			t.Fatalf("%s: no original backup in %+v", id, bs)
		}
		Set(home, id, API{BaseURL: "https://other.example.com", APIKey: "k", Model: "m", Protocol: a.protocols[0]})
		if err := Restore(home, id, orig); err != nil {
			t.Fatalf("%s: restore: %v", id, err)
		}
		for _, rel := range a.files {
			data, err := os.ReadFile(filepath.Join(home, rel))
			want, had := userFiles[rel]
			switch {
			case !had && err == nil:
				t.Errorf("%s: %s was made and not removed on restore", id, rel)
			case had && string(data) != want:
				t.Errorf("%s: %s restored as\n%s\nwant\n%s", id, rel, data, want)
			}
		}
	}
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestSessionsGetTheKeysOfAgentsThatReadThemFromTheEnvironment(t *testing.T) {
	home := t.TempDir()
	if err := Set(home, "codex", API{BaseURL: "https://gw.example.com/v1", APIKey: "sk-codex", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if env := LaunchEnv(home, "codex"); len(env) != 1 || env[0] != codexKeyVar+"=sk-codex" {
		t.Fatalf("codex env %q", env)
	}
	if err := Set(home, "copilot", API{BaseURL: "https://gw.example.com/v1", APIKey: "sk-cp", Model: "m", Protocol: Anthropic}); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(LaunchEnv(home, "copilot"), " ")
	if !strings.Contains(env, "COPILOT_PROVIDER_TYPE=anthropic") || !strings.Contains(env, "COPILOT_PROVIDER_API_KEY=sk-cp") || strings.Contains(env, protocolVar) {
		t.Fatalf("copilot env %s", env)
	}
	if err := Set(home, "continue", API{BaseURL: "https://gw.example.com/v1", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if args := LaunchArgs(home, "continue"); len(args) != 2 || args[0] != "--config" {
		t.Fatalf("continue args %q", args)
	}
	if fi, _ := os.Stat(filepath.Join(home, envFile("codex"))); fi.Mode().Perm() != 0o600 {
		t.Errorf("the keys' file is %v", fi.Mode().Perm())
	}
	if err := Set(home, "codex", API{BaseURL: "ftp://x"}); err == nil {
		t.Error("an address that isn't http was taken")
	}
	if err := Set(home, "codex", API{BaseURL: "https://x", Protocol: Anthropic}); err == nil {
		t.Error("a protocol codex doesn't speak was taken")
	}
}
