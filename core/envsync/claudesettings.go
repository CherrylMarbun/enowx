package envsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// Claude Code's settings.json says what its tools may do without asking
// (permissions) and what runs on its events (hooks). The Settings page
// edits both as lists; the rest of the file stays as it was, in order.

// ClaudePermissions are settings.json's permissions: rules such as
// "Bash(npm run test:*)" or "Read(./.env)".
type ClaudePermissions struct {
	Allow       []string `json:"allow,omitempty"`
	Ask         []string `json:"ask,omitempty"`
	Deny        []string `json:"deny,omitempty"`
	DefaultMode string   `json:"defaultMode,omitempty"` // "default", "acceptEdits", "plan", "bypassPermissions"
	// Directories are folders beyond the project the tools may reach.
	Directories []string `json:"additionalDirectories,omitempty"`
}

// ClaudeHook is one hook: on Event, for tools matching Matcher (a tool
// name or a pattern; "" for all), it runs Command, up to Timeout seconds.
// Hook keeps the hook's own fields, so a kind enowX doesn't edit stays.
type ClaudeHook struct {
	Event   string
	Matcher string
	Command string
	Timeout int
	Hook    map[string]any
}

// ClaudeHookEvents are the events hooks run on, in Claude Code's order.
var ClaudeHookEvents = []string{
	"PreToolUse", "PostToolUse", "UserPromptSubmit", "Notification", "Stop",
	"SubagentStop", "PreCompact", "SessionStart", "SessionEnd",
}

// ReadClaudeSettings reads a settings.json's permissions and hooks.
func ReadClaudeSettings(data []byte) (ClaudePermissions, []ClaudeHook, error) {
	var file struct {
		Permissions ClaudePermissions `json:"permissions"`
		Hooks       map[string][]struct {
			Matcher string           `json:"matcher"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	clean, _ := stripJSONC(data)
	if len(bytes.TrimSpace(clean)) == 0 {
		return ClaudePermissions{}, nil, nil
	}
	if err := json.Unmarshal(clean, &file); err != nil {
		return ClaudePermissions{}, nil, fmt.Errorf("envsync: settings.json: %w", err)
	}
	var hooks []ClaudeHook
	events := append(slices.Clone(ClaudeHookEvents), sortedKeys(file.Hooks)...)
	seen := map[string]bool{}
	for _, ev := range events {
		if seen[ev] {
			continue
		}
		seen[ev] = true
		for _, m := range file.Hooks[ev] {
			for _, h := range m.Hooks {
				hook := ClaudeHook{Event: ev, Matcher: m.Matcher, Hook: h}
				hook.Command, _ = h["command"].(string)
				if t, ok := h["timeout"].(float64); ok {
					hook.Timeout = int(t)
				}
				hooks = append(hooks, hook)
			}
		}
	}
	return file.Permissions, hooks, nil
}

// WriteClaudeSettings sets a settings.json's permissions and hooks,
// keeping its other keys where they were; empty ones leave the file.
func WriteClaudeSettings(data []byte, perm ClaudePermissions, hooks []ClaudeHook) ([]byte, error) {
	clean, _ := stripJSONC(data)
	if len(bytes.TrimSpace(clean)) == 0 {
		clean = []byte("{}")
	}
	top, err := objectMembers(clean)
	if err != nil {
		return nil, fmt.Errorf("envsync: settings.json: %w", err)
	}
	set := func(key string, value any, empty bool) error {
		if empty {
			top = slices.DeleteFunc(top, func(m member) bool { return m.key == key })
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		top = setMember(top, key, raw)
		return nil
	}
	permEmpty := len(perm.Allow)+len(perm.Ask)+len(perm.Deny)+len(perm.Directories) == 0 && perm.DefaultMode == ""
	if err := set("permissions", perm, permEmpty); err != nil {
		return nil, err
	}
	type group struct {
		Matcher string           `json:"matcher,omitempty"`
		Hooks   []map[string]any `json:"hooks"`
	}
	byEvent := map[string][]group{}
	var order []string
	for _, h := range hooks {
		entry := map[string]any{}
		for k, v := range h.Hook {
			entry[k] = v
		}
		if _, ok := entry["type"]; !ok {
			entry["type"] = "command"
		}
		if entry["type"] == "command" {
			entry["command"] = h.Command
			delete(entry, "timeout")
			if h.Timeout > 0 {
				entry["timeout"] = h.Timeout
			}
		}
		groups := byEvent[h.Event]
		if !slices.Contains(order, h.Event) {
			order = append(order, h.Event)
		}
		if i := slices.IndexFunc(groups, func(g group) bool { return g.Matcher == h.Matcher }); i >= 0 {
			groups[i].Hooks = append(groups[i].Hooks, entry)
		} else {
			groups = append(groups, group{Matcher: h.Matcher, Hooks: []map[string]any{entry}})
		}
		byEvent[h.Event] = groups
	}
	var ms []member
	for _, ev := range order {
		raw, err := json.Marshal(byEvent[ev])
		if err != nil {
			return nil, err
		}
		ms = append(ms, member{ev, raw})
	}
	if len(ms) == 0 {
		top = slices.DeleteFunc(top, func(m member) bool { return m.key == "hooks" })
	} else {
		top = setMember(top, "hooks", encodeObject(ms))
	}
	var out bytes.Buffer
	if err := json.Indent(&out, encodeObject(top), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}
