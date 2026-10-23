package agent

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// fakeBin is an ENOWX_BIN that records its arguments in log.
func fakeBin(t *testing.T, log string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "enowx")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$@\" >> "+log+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestCodexNotifyReportsDoneAndRunsTheUsersOwn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks are POSIX shell")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	user := []string{"sh", "-c", `echo "user $1 $2" >> ` + log, "x", "turn-ended"}
	args := codexHookArgs(user)
	if len(args) != 2 || args[0] != "-c" {
		t.Fatalf("args = %q", args)
	}
	// Codex parses the value as TOML.
	var cfg struct {
		Notify []string `toml:"notify"`
	}
	if _, err := toml.Decode(args[1], &cfg); err != nil {
		t.Fatalf("%s: %v", args[1], err)
	}
	// Codex runs notify with the turn as its last argument.
	cmd := exec.Command(cfg.Notify[0], append(cfg.Notify[1:], `{"type":"agent-turn-complete"}`)...)
	cmd.Env = append(os.Environ(), EnvBin+"="+fakeBin(t, log))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	got, _ := os.ReadFile(log)
	want := "hook done\nuser turn-ended {\"type\":\"agent-turn-complete\"}\n"
	if string(got) != want {
		t.Fatalf("ran:\n%s\nwant:\n%s", got, want)
	}

	// Without ENOWX_BIN (outside enowX) and without a notify of the user's,
	// it does nothing and succeeds.
	cfg.Notify = nil
	if _, err := toml.Decode(codexHookArgs(nil)[1], &cfg); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cfg.Notify[0], append(cfg.Notify[1:], "{}")...).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestClaudeHooksReportEachState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks are POSIX shell")
	}
	args := claudeHookArgs()
	if len(args) != 2 || args[0] != "--settings" {
		t.Fatalf("args = %q", args)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct{ Type, Command string }
		}
	}
	if err := json.Unmarshal([]byte(args[1]), &settings); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "log")
	bin := fakeBin(t, log)
	for _, event := range []string{"UserPromptSubmit", "Notification", "Stop"} {
		for _, m := range settings.Hooks[event] {
			for _, h := range m.Hooks {
				cmd := exec.Command("sh", "-c", h.Command)
				cmd.Env = append(os.Environ(), EnvBin+"="+bin)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%s: %v: %s", event, err, out)
				}
			}
		}
	}
	got, _ := os.ReadFile(log)
	if want := "hook working\nhook waiting\nhook done\n"; string(got) != want {
		t.Fatalf("ran %q, want %q", got, want)
	}
	if !strings.Contains(settings.Hooks["Notification"][0].Matcher, "permission_prompt") {
		t.Fatalf("Notification matcher = %q", settings.Hooks["Notification"][0].Matcher)
	}
}
