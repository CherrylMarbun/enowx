package agent

import (
	"slices"
	"testing"
)

func TestArgv(t *testing.T) {
	cases := []struct {
		agent string
		l     Launch
		want  []string
	}{
		{"claude", Launch{Prompt: "fix it", SkipPermissions: true}, []string{"claude", "--dangerously-skip-permissions", "fix it"}},
		{"claude", Launch{SkipPermissions: true, Continue: true}, []string{"claude", "--continue", "--dangerously-skip-permissions"}},
		{"codex", Launch{SkipPermissions: true, Continue: true}, []string{"codex", "resume", "--last", "--dangerously-bypass-approvals-and-sandbox"}},
		{"gemini", Launch{Prompt: "hi"}, []string{"gemini", "--prompt-interactive", "hi"}},
		{"opencode", Launch{Prompt: "hi", SkipPermissions: true}, []string{"opencode", "--auto", "--prompt", "hi"}},
		{"kiro", Launch{SkipPermissions: true, Continue: true, Prompt: "ignored when continuing"}, []string{"kiro-cli", "chat", "--resume", "--trust-all-tools"}},
		{"copilot", Launch{Prompt: "hi", SkipPermissions: true}, []string{"copilot", "--yolo", "-i", "hi"}},
		{"grok", Launch{Prompt: "-v looks like a flag"}, []string{"grok", "--", "-v looks like a flag"}},
		{"hermes", Launch{Prompt: "hi"}, []string{"hermes", "--tui", "--query", "hi"}},
		{"rovo", Launch{SkipPermissions: true}, []string{"acli", "rovodev", "run", "--yolo"}},
		// Typed in once it shows its input, not on its command line.
		{"amp", Launch{Prompt: "hi", SkipPermissions: true}, []string{"amp", "--dangerously-allow-all"}},
	}
	for _, tc := range cases {
		p, ok := Find(tc.agent)
		if !ok {
			t.Fatalf("no preset %s", tc.agent)
		}
		if got := p.Argv(tc.l); !slices.Equal(got, tc.want) {
			t.Errorf("%s %+v: argv = %q, want %q", tc.agent, tc.l, got, tc.want)
		}
	}
}

func TestPresetsAreDistinct(t *testing.T) {
	seen := map[string]bool{Shell: true, Custom: true}
	for _, p := range Presets {
		if seen[p.ID] || p.Name == "" || p.Command == "" || p.Homepage == "" {
			t.Errorf("preset %+v: repeated ID, or no name, command or homepage", p)
		}
		seen[p.ID] = true
		if p.TypePrompt && len(p.PromptFlag) > 0 {
			t.Errorf("%s both types its prompt and passes it", p.ID)
		}
	}
	if len(Presets) < 40 {
		t.Fatalf("only %d agents", len(Presets))
	}
}

func TestEnvironSkipsThroughTheEnvironment(t *testing.T) {
	goose, _ := Find("goose")
	if env := goose.Environ(true); !slices.Contains(env, "GOOSE_MODE=auto") {
		t.Fatalf("skipping: %q", env)
	}
	if env := goose.Environ(false); len(env) != 0 {
		t.Fatalf("not skipping: %q", env)
	}
}
