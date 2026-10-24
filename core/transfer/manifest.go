package transfer

import (
	"regexp"
	"time"
)

// An archive holds, in this order: the manifest, then the project's files
// under project/, then each session's under sessions/<id>/: its screen
// (history.vt), its agent's conversation (transcript.jsonl), and for a
// worktree session the worktree's files (worktree/).
const (
	ManifestName  = "manifest.json"
	ProjectPrefix = "project"
	SessionPrefix = "sessions"
)

// Version is the archive's layout; a daemon refuses one newer than it.
const Version = 1

// Manifest says what an archive carries.
type Manifest struct {
	Version  int       `json:"version"`
	Name     string    `json:"name"` // the project's
	Path     string    `json:"path"` // its folder on the source host
	Env      bool      `json:"env,omitempty"`
	Sessions []Session `json:"sessions,omitempty"`
}

// Session is a session as an archive carries it: what starts it again.
type Session struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
	// ParentKey is its parent's key (projectsync.Key), for a sync, where
	// the parent may be on the other host already.
	ParentKey string    `json:"parentKey,omitempty"`
	Name      string    `json:"name,omitempty"`
	Prompt    string    `json:"prompt,omitempty"`
	Agent     string    `json:"agent,omitempty"`
	Command   []string  `json:"command,omitempty"` // a custom command's
	Branch    string    `json:"branch,omitempty"`
	Worktree  bool      `json:"worktree,omitempty"`
	Dir       string    `json:"dir"` // on the source host
	Skip      bool      `json:"skipPermissions,omitempty"`
	Budget    int64     `json:"budget,omitempty"` // seconds
	Created   time.Time `json:"created"`
	// Conversation is the agent's id of its conversation (Claude Code's
	// session id), whose transcript the archive carries; "" for none.
	Conversation string `json:"conversation,omitempty"`
}

// RewritePaths replaces the folder from with to in a transcript, where
// the agent recorded where it ran: whole path segments only, so /a/app
// leaves /a/app2 alone.
func RewritePaths(data []byte, from, to string) []byte {
	if from == "" || from == to {
		return data
	}
	re := regexp.MustCompile(regexp.QuoteMeta(from) + `([/"\\]|$)`)
	return re.ReplaceAll(data, []byte(escapeRepl(to)+"$1"))
}

// escapeRepl escapes $ in a replacement.
func escapeRepl(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '$' {
			out = append(out, '$')
		}
		out = append(out, s[i])
	}
	return string(out)
}
