package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/enowx/enowx/proto"
)

// EnvBin names the variable holding the program the agents' hooks run, as
// "$ENOWX_BIN hook <state>". The daemon sets it in every session, so a
// command saved for a restart keeps working when enowX moves or updates.
const EnvBin = "ENOWX_BIN"

// hookCommand is the shell command a hook runs to report state. Outside
// enowX (no ENOWX_BIN) it does nothing.
func hookCommand(state string) string {
	return `[ -z "$` + EnvBin + `" ] || "$` + EnvBin + `" hook ` + state
}

// HookArgs returns the arguments that make agent id report its state
// (proto.State*) through its own hooks, or nil when enowX has none for it.
// They add to the user's configuration and replace none of it. See
// docs/agents.md.
func HookArgs(id string) []string {
	if !hooksSupported() {
		return nil
	}
	switch id {
	case "claude":
		return claudeHookArgs()
	case "codex":
		return codexHookArgs(codexNotify())
	}
	return nil
}

// claudeHookArgs passes Claude Code hooks in --settings, which it merges
// with the user's settings: a prompt sent and a tool run mean working,
// a permission or a question asked means waiting, the end of a turn done.
func claudeHookArgs() []string {
	type hook struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	type matcher struct {
		Matcher string `json:"matcher,omitempty"`
		Hooks   []hook `json:"hooks"`
	}
	on := func(match, state string) []matcher {
		return []matcher{{Matcher: match, Hooks: []hook{{Type: "command", Command: hookCommand(state), Timeout: 5}}}}
	}
	settings := map[string]any{"hooks": map[string][]matcher{
		"UserPromptSubmit": on("", proto.StateWorking),
		"PostToolUse":      on("", proto.StateWorking),
		"Notification":     on("permission_prompt|elicitation_dialog", proto.StateWaiting),
		"Stop":             on("", proto.StateDone),
	}}
	return []string{"--settings", marshal(settings)}
}

// codexHookArgs sets Codex's notify, which it runs at the end of each turn
// with the turn as a last argument, to report done and then run the user's
// own notify command, if any, as Codex would have.
func codexHookArgs(user []string) []string {
	script := hookCommand(proto.StateDone) + `; [ $# -lt 2 ] || exec "$@"`
	argv := append([]string{"/bin/sh", "-c", script, "enowx-notify"}, user...)
	// A JSON array of strings is a TOML array too.
	return []string{"-c", "notify=" + marshal(argv)}
}

// codexNotify is the notify command of the user's Codex configuration.
func codexNotify() []string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		home = filepath.Join(h, ".codex")
	}
	var cfg struct {
		Notify []string `toml:"notify"`
	}
	if _, err := toml.DecodeFile(filepath.Join(home, "config.toml"), &cfg); err != nil {
		return nil
	}
	return cfg.Notify
}

// marshal is JSON without HTML escapes, which TOML would keep literally.
func marshal(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSpace(b.String())
}
