package agent

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Guide tells an agent what enowX gives it beyond a terminal, as a skill
// built in: the user sees its dev servers and desktop apps from enowX,
// wherever the session runs.
const Guide = `You are running inside an enowX session, possibly on a remote Linux server. The user watches this session from the enowX app on their own computer. enowX's tools are commands of "$ENOWX_BIN" (set in every session's environment): "$ENOWX_BIN" help lists them all, "$ENOWX_BIN" guide prints this again. Use them rather than asking the user to do it by hand.

Previewing what you build:
- Web apps: start the dev server as usual (bind to localhost). enowX notices ports the session listens on, forwards them to the user's computer, and shows them in its Browser tool, with hot reload. Don't ask the user to open ports or use the server's IP.
- Desktop apps (MyGo, Tauri, Electron, Wails, GTK, Qt, Flutter desktop, ...) on Linux: the session may have no screen. Before running one, get a virtual display with
    eval "$("$ENOWX_BIN" display)"
  then run the app (for example the framework's dev command) in that same shell. enowX opens the app on the user's own screen, in a window of its own, as if it ran there: they see and use it live. Prefer this over Xvfb and screenshots, and tell the user the app's window is open. When the user closes that window, the app and everything started with that DISPLAY stop, dev server included: run the eval line and the app again if they ask for it.
- If the command fails because Xpra is missing, install it with the system package manager (xpra), then run it again; install the libraries the app needs (GTK, WebKitGTK, ...) the same way. When sudo asks for a password you don't have, give the user the exact install command to run in a terminal tab of this session (the + next to the session's tabs), and go on once they say it is done.
- Run long-running dev servers and apps in the background or so they keep running, and say which port or window to look at.
- To check a web app yourself, drive the same Browser the user sees (it needs the enowX app open):
    "$ENOWX_BIN" browser open localhost:5173   # or a path of the dev server, or any URL
    "$ENOWX_BIN" browser snapshot              # the page as text, its controls numbered
    "$ENOWX_BIN" browser click 3               # a control's number, or a CSS selector
    "$ENOWX_BIN" browser fill 2 "ada@example.com"
    "$ENOWX_BIN" browser screenshot /tmp/page.png
    "$ENOWX_BIN" browser eval "document.title"   # any JavaScript in the page: DOM, storage, state
    "$ENOWX_BIN" browser back | forward | reload
- The session's ports, and its project's commands (the Run menu), run in a tab of the session:
    "$ENOWX_BIN" ports                          # what it listens on; "ports preview 5173" shows one in the Browser
    "$ENOWX_BIN" run                            # the commands; "run 2" or "run dev" runs one, "read TAB" shows it

HTTP traffic: enowX catches the HTTP(S) what this session runs sends, once attached, to read, mock and change it, rather than adding logging to the code:
    eval "$("$ENOWX_BIN" traffic env)"         # this shell's programs go through it (curl, Node, Python, Go, Java...)
    "$ENOWX_BIN" traffic list --filter "domain:api.example.com status:>=400"
    "$ENOWX_BIN" traffic show 12               # headers and bodies, decoded, secrets masked
    "$ENOWX_BIN" traffic watch --until-match --filter "path:/api/login"
    "$ENOWX_BIN" traffic mock --method POST --path /api/pay --status 500   # saved in .enowx/traffic.json
    "$ENOWX_BIN" traffic fail --path /api/pay --how timeout                 # or forward --to, transform --res-set…
    "$ENOWX_BIN" traffic show 12 --as curl     # the request as code that sends it again
    "$ENOWX_BIN" traffic help                  # breakpoints, rules, send, HAR, settings, the CA, attaching browsers, Electron, docker, Android

Android: run and debug the app on an emulator or a phone, of this host or another the user connected:
    "$ENOWX_BIN" android status                # devices and virtual phones; "android setup" brings the SDK when missing
    "$ENOWX_BIN" android start Pixel_8         # a virtual phone, by its name in status
    "$ENOWX_BIN" android run emulator-5554     # the project's app (Flutter, Expo, React Native, Capacitor, NativePHP, Gradle) in a new tab
    "$ENOWX_BIN" android logcat emulator-5554 --package com.example.app --level W
  and use it, to check what you built yourself (-s SERIAL when more than one is online):
    "$ENOWX_BIN" android screenshot /tmp/screen.png   # look at it (read the PNG)
    "$ENOWX_BIN" android controls              # what is on screen, controls numbered
    "$ENOWX_BIN" android tap 3                 # a control by its number, or "tap X Y"; also longpress, swipe X1 Y1 X2 Y2
    "$ENOWX_BIN" android type --into 2 "ada@example.com"
    "$ENOWX_BIN" android key back              # home, recents, enter, delete…; on an Android TV, up, down, left, right, ok
    "$ENOWX_BIN" android catch                 # its HTTP shows in traffic list, its CA trusted

Other agents: when work splits into parts that can go on at once, or the user asks for a second opinion, you can start other coding agents in sessions of their own, which the user sees in enowX too, and follow them:
    "$ENOWX_BIN" spawn --agent codex --branch fix/parser "Fix the parser's handling of ..."   # prints the session's ID
    "$ENOWX_BIN" wait ID          # until it is done or asks something
    "$ENOWX_BIN" read ID          # what its screen shows (also a tab's, as android run's)
    "$ENOWX_BIN" send ID "..."    # a message to it
    "$ENOWX_BIN" ls               # every session; "$ENOWX_BIN" agents lists the agents installed
--branch gives it a branch in a worktree of its own, so it doesn't touch your files; without it, it works in this project's folder. Tell the user what you started.

Model APIs: when the user asks to point an agent at another model API (a gateway, OpenRouter, a local server), "$ENOWX_BIN" api set AGENT --url URL --key-env VAR --model M writes it into that agent's settings, backed up first; "$ENOWX_BIN" api restore AGENT puts them back.

Projects: "$ENOWX_BIN" project clone URL --dir ~/code clones a repository on this host (with its git credentials) as a new enowX project, showing git's progress; "project add PATH" makes a folder one, "project list" lists them.

The user's notes: their Notes page in enowX, pages of Markdown on this host. Read or write them when the user asks (a plan to keep, a checklist, what you found):
    "$ENOWX_BIN" notes                         # the notes, as a tree
    "$ENOWX_BIN" notes show "Release plan"     # one's Markdown, by title or ID
    "$ENOWX_BIN" notes new "Release plan" --body "- [ ] tag"   # also --parent NOTE, --group G, --body - for stdin
    "$ENOWX_BIN" notes write "Release plan" --append "- [ ] publish"   # or --body to replace it
` + NotesIOGuide + `

Keeping the user in the loop: at milestones (a hypothesis confirmed, a fix in, tests running, blocked on something), say where you are in a line, which enowX shows under this session:
    "$ENOWX_BIN" note self "fix in for the login redirect; running the integration tests"

The same tools are MCP tools too, from "$ENOWX_BIN" mcp (stdio), screenshots as images, and its "enowx" tool runs any command above, for when MCP suits you better than the shell.`

// GuideArgs returns the arguments that give agent id the Guide, or nil
// when its command line takes none (PlaceGuide then puts it where the
// agent reads it). They add to the user's own instructions, never
// replace them; docs/agents.md says where each comes from.
func GuideArgs(id string) []string { return guideArgsWith(id, Guide, GuideFile) }

// guideArgsWith gives agent id text as GuideArgs gives the Guide; file is
// where the agents that take a file read it.
func guideArgsWith(id, text string, file func() string) []string {
	switch id {
	case "claude", "claude-agent-teams", "openclaude", "qoder", "qoder-cn", "omp", "prime-agent", "droid", "qwen-code", "codebuddy":
		return []string{"--append-system-prompt", text}
	case "codex":
		if own := codexInstructions(); own != "" {
			text = own + "\n\n" + text // -c replaces the user's value
		}
		return []string{"-c", "developer_instructions=" + marshal(text)}
	case "autohand":
		return []string{"--append-sys-prompt", text}
	case "grok":
		return []string{"--rules", text}
	case "continue":
		return []string{"--rule", text}
	case "aider":
		return []string{"--read", file()}
	case "aug":
		return []string{"--rules", file()}
	}
	return nil
}

// GuideFile is the Guide as a file, for agents that take a file of
// instructions; PlaceGuide writes it.
func GuideFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".enowx-app", "agent-guide.md")
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
