package agent

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Guide tells an agent what enowX gives it beyond a terminal, as a skill
// built in: the user sees its dev servers and desktop apps from enowX,
// wherever the session runs.
const Guide = `You are running inside an enowX session, possibly on a remote Linux server. The user watches this session from the enowX app on their own computer.

Previewing what you build:
- Web apps: start the dev server as usual (bind to localhost). enowX notices ports the session listens on, forwards them to the user's computer, and shows them in its Browser tool, with hot reload. Don't ask the user to open ports or use the server's IP.
- Desktop apps (MyGo, Tauri, Electron, Wails, GTK, Qt, Flutter desktop, ...) on Linux: the session may have no screen. Before running one, get a virtual display with
    eval "$("$ENOWX_BIN" display)"
  then run the app (for example the framework's dev command) in that same shell. enowX opens the app on the user's own screen, in a window of its own, as if it ran there: they see and use it live. Prefer this over Xvfb and screenshots, and tell the user the app's window is open. When the user closes that window, the app and everything started with that DISPLAY stop, dev server included: run the eval line and the app again if they ask for it.
- If the command fails because Xpra is missing, install it with the system package manager (xpra), then run it again; install the libraries the app needs (GTK, WebKitGTK, ...) the same way. When sudo asks for a password you don't have, give the user the exact install command to run in a terminal tab of this session (the + next to the session's tabs), and go on once they say it is done.
- Run long-running dev servers and apps in the background or so they keep running, and say which port or window to look at.

Other agents: when work splits into parts that can go on at once, or the user asks for a second opinion, you can start other coding agents in sessions of their own, which the user sees in enowX too, and follow them:
    "$ENOWX_BIN" spawn --agent codex --branch fix/parser "Fix the parser's handling of ..."   # prints the session's ID
    "$ENOWX_BIN" wait ID          # until it is done or asks something
    "$ENOWX_BIN" read ID          # what its screen shows
    "$ENOWX_BIN" send ID "..."    # a message to it
    "$ENOWX_BIN" ls               # every session; "$ENOWX_BIN" agents lists the agents installed
--branch gives it a branch in a worktree of its own, so it doesn't touch your files; without it, it works in this project's folder. Tell the user what you started.`

// GuideArgs returns the arguments that give agent id the Guide, or nil
// when enowX knows no way to. They add to the user's own instructions.
func GuideArgs(id string) []string {
	switch id {
	case "claude", "claude-teams":
		return []string{"--append-system-prompt", Guide}
	case "codex":
		text := Guide
		if own := codexInstructions(); own != "" {
			text = own + "\n\n" + Guide // -c replaces the user's value
		}
		return []string{"-c", "developer_instructions=" + marshal(text)}
	}
	return nil
}

// codexInstructions is developer_instructions of the user's Codex
// configuration.
func codexInstructions() string {
	var cfg struct {
		Instructions string `toml:"developer_instructions"`
	}
	if _, err := toml.DecodeFile(filepath.Join(codexHome(), "config.toml"), &cfg); err != nil {
		return ""
	}
	return cfg.Instructions
}

// codexHome is where Codex keeps its configuration.
func codexHome() string {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return home
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(h, ".codex")
}
