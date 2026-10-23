package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/proto"
)

// codexBuiltins are the commands of Codex 0.164's / menu, in its order.
// Codex has no way to list them outside its screen.
var codexBuiltins = [][2]string{
	{"model", "choose what model and reasoning effort to use"},
	{"presets", "run or save reusable prompts and slash commands"},
	{"ide", "include current selection, open files, and other context from your IDE"},
	{"permissions", "choose what Codex is allowed to do"},
	{"keymap", "remap TUI shortcuts"},
	{"vim", "toggle Vim mode for the composer"},
	{"experimental", "toggle experimental features"},
	{"approve", "approve one retry of a recent auto-review denial"},
	{"memories", "configure memory use and generation"},
	{"skills", "use skills to improve how Codex performs specific tasks"},
	{"import", "import setup, this project, and recent chats from Claude Code"},
	{"hooks", "view and manage lifecycle hooks"},
	{"review", "review my current changes and find issues"},
	{"rename", "rename the current thread"},
	{"new", "start a new chat during a conversation"},
	{"archive", "archive this session"},
	{"delete", "permanently delete this session"},
	{"resume", "resume a saved chat"},
	{"fork", "fork the current chat"},
	{"worktree", "start or continue a conversation in a new worktree"},
	{"init", "create an AGENTS.md file with instructions for Codex"},
	{"compact", "summarize conversation to prevent hitting the context limit"},
	{"config", "configure Codex preferences"},
	{"recap", "summarize the current conversation now"},
	{"plan", "switch to Plan mode"},
	{"goal", "set or view the goal for a long-running task"},
	{"agents", "open the agent command center"},
	{"side", "start a side conversation in an ephemeral fork"},
	{"copy", "copy the last response or part of it"},
	{"export", "export the conversation as markdown"},
	{"raw", "toggle raw scrollback mode for copy-friendly terminal selection"},
	{"diff", "show git diff (including untracked files)"},
	{"mention", "mention a file"},
	{"status", "show current session configuration and token usage"},
	{"warnings", "view retained warnings and diagnostic details"},
	{"cd", "change the current working directory"},
	{"pwd", "show the current working directory"},
	{"usage", "view account usage or use a usage limit reset"},
	{"statusline", "configure which items appear in the status line"},
	{"theme", "choose a syntax highlighting theme"},
	{"mcp", "list MCP tools; use /mcp verbose or /mcp login <name>"},
	{"plugins", "browse plugins"},
	{"ps", "list background terminals"},
	{"stop", "stop all background terminals"},
	{"clear", "clear the terminal and start a new chat"},
	{"subagents", "switch between this session's subagents"},
	{"feedback", "send logs to maintainers"},
	{"logout", "log out of Codex"},
	{"exit", "exit Codex"},
}

// codexCommands are Codex's own commands and the user's custom prompts
// (~/.codex/prompts/<name>.md, run as /prompts:<name>).
func codexCommands() []proto.AgentCommand {
	list := make([]proto.AgentCommand, 0, len(codexBuiltins))
	for _, b := range codexBuiltins {
		list = append(list, proto.AgentCommand{Name: b[0], Description: b[1], Kind: "built-in"})
	}
	home, _ := os.UserHomeDir()
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	notes := map[string]string{}
	folder := filepath.Join(codexHome, "prompts")
	readNotes(folder, "", notes)
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".md"); ok && !e.IsDir() {
			list = append(list, proto.AgentCommand{Name: "prompts:" + name, Description: notes[name], Kind: "command"})
		}
	}
	return list
}
