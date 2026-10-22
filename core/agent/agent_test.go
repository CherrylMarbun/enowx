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
