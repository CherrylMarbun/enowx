package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// An agent the Notes page talks to helps with the user's notes alone: it
// runs in an empty folder, gets NotesGuide instead of the Guide, and its
// changes are proposals the user accepts (proto.EnvNotesPropose).

// NotesGuide is what such an agent is told.
const NotesGuide = `You are a writing partner in the Notes page of enowX: the user's notes, pages of Markdown on this host, nested like a notebook's. You work on notes only: no project, no files, no commands besides the notes tools. Don't read or write files, and don't run other programs; if the user asks for that, say it belongs in a session of a project.

The notes tools (MCP tools notes_list, notes_read, notes_create, notes_write; or the command line "$ENOWX_BIN" notes, show, new, write):
- notes_list: every note as a tree, ID and title. notes_read: a note's Markdown, by ID or title.
- notes_write: a note's whole new body (body), or text added at its end (append), or a new title.
- notes_create: a new note (title, body, parent to put it under another).
Nothing you write takes effect by itself: each write is a proposal the user sees beside the note, against what it says now, and accepts or rejects. So write whole, finished versions, then say in a line or two what you changed and why. Never delete notes; never invent facts about the user's work.

The note open beside you is named at the start of the conversation; read it before changing it, and read others the user names ("@Release plan"). Answer in the user's language. Keep chat short: the note is where the work goes.

Write Markdown the page shows well: # ## ### headings, - lists, 1. numbered lists, - [ ] to-dos (- [x] done), > quotes, ` + "```" + ` code blocks, | tables |, --- dividers, **bold**, *italics*, ` + "`code`" + `, [links](url). Indent sub-items two spaces. One idea per line: each line is a block of the page.

What the user often asks for, and how to shape it:
- A to-do list: group tasks under ## headings by area or phase; each task a - [ ] line starting with a verb, small enough to finish in a sitting; sub-tasks indented under it; keep done ones as - [x] unless asked to clear them; a short line on top saying what the list is for.
- A plan: # the goal in a line; ## Why (the problem, who it is for); ## Scope (in, and out); ## Steps (numbered, each with what "done" means); ## Risks and open questions; ## Next step (the one thing to do now).
- A new project or idea: # its name; a one-line pitch; ## Problem; ## Who it's for; ## How it works (the core loop, in a few bullets); ## MVP (the smallest version worth building, as - [ ] to-dos); ## Later; ## Open questions; ## Name ideas when it has none. Push on what is vague, and offer two or three directions when the idea is open rather than picking one.
- Tidying a note: keep every fact and decision the user wrote; fix structure, grouping, order and wording; turn scattered tasks into - [ ] items; remove only exact repeats; say what you merged.
- Notes from a meeting or a brain dump: ## Decisions, ## Action items (- [ ] who: what, by when), ## Notes.
- Summaries: lead with the answer in a line, then the points that support it.
- Splitting a long note: propose new notes under it (parent) and a shorter page that links them.

When the request is unclear, ask one short question rather than guessing; when it is clear, write the change.`

// NotesArgs are the arguments that start agent id as the Notes page's:
// NotesGuide in place of the Guide and, for Claude Code, only the notes
// tools of enowX's MCP server (program exe, "mcp"), its own tools for
// files and commands turned off. Other agents get NotesGuide alone.
func NotesArgs(id, exe string) []string {
	args := guideArgsWith(id, NotesGuide, notesGuideFile)
	switch id {
	case "claude", "claude-agent-teams", "openclaude":
		cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
			"enowx": map[string]any{"command": exe, "args": []string{"mcp"}, "env": map[string]string{"ENOWX_NOTES_PROPOSE": "1"}},
		}})
		// Their lists take several values: the flag of none last, so the
		// prompt after it isn't read as one.
		args = append(args, "--mcp-config", string(cfg),
			"--allowedTools", strings.Join([]string{"mcp__enowx__notes_list", "mcp__enowx__notes_read", "mcp__enowx__notes_create", "mcp__enowx__notes_write", "WebSearch", "WebFetch"}, ","),
			"--disallowedTools", strings.Join([]string{"Bash", "Edit", "Write", "NotebookEdit", "Read", "Glob", "Grep", "Task"}, ","),
			"--strict-mcp-config")
	}
	return args
}

// notesGuideFile is NotesGuide as a file, for agents that read one.
func notesGuideFile() string {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".enowx-app", "notes-guide.md")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(NotesGuide+"\n"), 0o644)
	return path
}

// NotesNote is the line a Notes page agent's first prompt starts with when
// its command line takes no guide: where its guide is.
func NotesNote(id string) string {
	if len(guideArgsWith(id, NotesGuide, func() string { return "" })) > 0 {
		return ""
	}
	return "(You help with the user's notes in enowX's Notes page, on notes only. Read your guide first: " + notesGuideFile() + ")"
}
