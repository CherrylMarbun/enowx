# Changelog

Each version's section is also what the app's update dialog shows.

## 0.4.0

- **enowX Cloud, for teams (opt-in):** sign in with GitHub and make a team, or join one from an invite link, in the app or right in the browser before you have it. Nothing reaches the cloud until you sign in, and what a team shares is end-to-end encrypted: the cloud keeps ciphertext only, never credentials.
- **A team's space:** its notes and canvases beside your own (switch in the account menu, move or copy between them), its projects as Git links, cloned in one click or found when you have them, its kit of skills, commands, subagents, pipelines, automations and templates, and each project's rules for the agents.
- **Shared hosts:** share a host with your team over Tailscale; members see its sessions, watch one live, ask to take it over (you let them and take it back), or start their own there with their own agent account, never yours. Keep a session private from its menu.
- **Team chat** in the side panel's Team group: `@` mentions members, notes, canvases, projects and files, Enter sends, messages edit (Up edits your last) and delete. The group also lists the team's projects, where each is and the sessions in it, and its members, who is on your host.
- **Team admin:** roles, invites, an activity log of who entered which host and started or took over which session, and a recovery key for the team's keys should every device be lost.
- **InstaBox:** create a VM there from Create a server, or connect to a machine you have, reached through its SSH gateway with a certificate signed for your key each time; paused machines wake as you connect. Its API key lives in the vault.
- **Discord:** what you work on shows under your name, the project, branch and agent with a Get enowX button; Settings → Discord turns it off or hides the names.
- Fixed: an event raised right as a client connected could be missed, so an agent asking to turn a feature on could wait for an answer that never came.

## 0.3.2

- **Terminal font:** Settings → Terminal picks the font terminals draw with, among the monospaced fonts of this device. Pick a Nerd Font and the icons programs like OMP print show as icons, where the system's monospaced font drew them as boxes. Terminals already open change at once.

## 0.3.1

- **Light and System appearance:** choose Dark, Light or System in Settings → Appearance; every palette has a light variant, your palette and accent are kept, and terminals stay dark. Until you pick one, web pages follow your device as before. Text on a custom accent now takes black or white, whichever reads better.
- **Images in Notes:** add a picture from a file or paste a screenshot, write beside it, mention it elsewhere on the page with `@`, rename or remove it, and undo or redo any edit (Ctrl/⌘+Z). Pictures nothing names any more are cleaned up after a week.
- **New session goes on with one you have:** pick a session of the project, running, hibernated or a past conversation, to open, wake or resume it, with an optional message.
- **OMP:** continue and resume its sessions, auto-approve, and its conversations in the chat view; sessions in one folder keep their own.
- **Files:** copy an opened file's source text from its toolbar, Markdown and HTML previews included.
- **Canvas:** zoomed out below 50%, a mockup shows as a picture of itself, so it shrinks whole instead of reflowing; click it to zoom in.
- **Sync with another host** shows each step while connecting, the Tailscale check's link to open, and can be cancelled; a server that stops answering while signing in gives up instead of leaving "Connecting…" forever.
- Fixed: after a project sync, a running agent goes on with the conversation brought instead of its old one; one file too large to sync no longer stops your data syncing (it is skipped and counted); large syncs no longer drop the connection; on Windows the app no longer restarts its daemon, and ends its sessions, at every connection; a session continued, woken or restored after a restart goes on with its own conversation, not the folder's latest.

## 0.3.0

- **Canvas:** design mockups, wireframes, diagrams and boards with agents on an endless board (scroll to move, pinch to zoom), watch them built live, compare and star variations, click a component to ask about it, change or redesign it, lay designs on a grid, share a read-only link on your tailnet, and turn a design into code. Agents start diagrams on a sheet of their own and keep designs within their frame.
- **Notes:** structured pages (tabs, toggles, columns, callouts, cards, an outline), version history, SVG icons in any color and size, an agent beside each page (Claude Code, Codex…) whose changes you accept, notes attached to projects on any host, and import from Notion (also on a schedule), Obsidian, Evernote, Keep and HTML, export to Markdown, HTML, PDF or a backup.
- **Credential vault:** your logins kept encrypted on this device and lent to sessions on any host when you allow it. Fill it from what a host's tools already keep, a token copied on the platform's page, or signing in to GitHub in the browser; git, gh, docker and ssh on servers use it.
- **Your data on every host:** notes, canvases, settings and the vault sync between this device and your servers, merged when both changed, conflicts kept to pick.
- **Projects across hosts:** clone from Git (also onto another host), keep a project's copies in sync on their own with conflicts announced, and auto push to a branch of its own.
- **Sounds:** a sound when an agent finishes, needs you or fails, your own sounds too, quiet hours and a count on the app's icon.
- **Features:** turn what agents are told about on or off, everywhere or per host, project or session.
- **Traffic:** pause, stop and resume recording. **Hibernate** a session yourself from its menu.
- **Floating control:** an optional radial menu over other apps (Cmd/Ctrl+Shift+Space): this PC or a VPS, a project, its sessions.
- Menus say a few words, the rest in a tooltip; Settings fills the window, and its sections no longer flicker between tabs and a menu.
- Fixed: the app's tests no longer stop a running daemon, Claude Code keeps its transcript when enowX was opened from an agent's terminal, a second daemon no longer takes the server's systemd service, the first auto push of a clean project works, and Windows passes agents their arguments whole in PowerShell 5 and 7.

## 0.2.2

- **Notes:** a Notes page in the rail with pages of notes on each host, nested like a notebook's. Write a block at a time: Markdown's shortcuts (`# `, `- `, `[] `, `> `) and `/` make headings, lists, to-dos, quotes, code, tables and dividers; drag blocks and pages to move them. Pages save themselves, and agents read and write them too (`enowx-app notes`).
- **Groups and order of projects:** drag projects and groups in the session list, or put projects in a group from their menu. The host keeps them, so every device shows the same list.
- **Pick your shell:** Settings → Terminal picks the shell terminals start with (zsh, bash, fish, PowerShell, Command Prompt, Git Bash, WSL…), and a session's **+** opens a tab of any of them.
- **Running as root:** a server's form now says whom sessions run as when you sign in as root, a regular user or root, and the choice sticks: connecting again no longer goes back to the regular user.
- Session rows stay two lines high; an agent's note shows on hovering its name.
- **Windows:** sessions start again (0.2.1's background daemon missed a library).
- Android: a device on the server connects through the server's adb, and the SDK is found by its adb.

## 0.2.1

- **The tray:** closing the window keeps enowX in the menu bar (macOS) or the notification area (Windows, Linux). Its icon counts the sessions that need you; a click lists those needing input, working or done, each opening the window on it. Settings → App can make closing quit again.
- **Android TV:** TVs are recognized; their screen fills the panel with a remote under it (D-pad, OK, Menu), Enter is OK, and agents press the remote's keys. **+ Device** connects a TV over the network, or makes a virtual one. Mi TVs' screenshots work.
- **Windows:** installing a new version while enowX runs no longer fails with "Error opening file for writing": the background daemon runs from a copy of the app. Updating from 0.2.0 may still need enowX ended once in Task Manager.

## 0.2.0

- **Traffic:** see the HTTP what a session runs sends and receives, HTTPS, WebSocket, SSE and gRPC included; mock, forward, change, fail or pause requests with rules; catch a terminal, a browser, an Electron app, Docker containers, the Browser tool or an Android device; export HAR. Its forms open as dialogs.
- **Android:** run the session's app on an emulator or a phone, of this machine or the server, read its log, and use its screen right in the side panel. Catching a device's traffic trusts the CA by itself on emulators and rooted phones.
- **Model APIs** (Settings): point Claude Code, Codex, OpenCode, Gemini CLI and 12 more agents at an API of yours (a gateway, OpenRouter, a local server), with several models and, where the agent takes them, vision, context window, output limit and reasoning effort. Each agent's settings are backed up first and restored in a click.
- **Every agent knows enowX's tools:** not only Claude Code and Codex. And agents can now do everything the side panel does: look at and tap an Android device, change traffic, run the project's commands, list its ports, set model APIs, from the command line or over MCP.
- The port cards can be turned off, in the Ports tool.
- Windows: no console window flashes for the programs enowX runs in the background.

## 0.1.1

- Windows: setting up a server no longer flashes a burst of console windows while this device's tools are scanned.
- The New session form fits small windows: its body scrolls, and Cancel and Start session stay in view.

Thanks to @Tiyouw for both fixes.

## 0.1.0

The first release.

- Run Claude Code, Codex and 43 other coding agents in parallel, each on its own branch in its own git worktree; fan out one prompt to several and keep the best; pipelines and automations.
- Watch them as terminals, or Claude Code and Codex as a chat; review each session's changes, comment lines back, commit, push, open a pull request and merge.
- Your own server over SSH or Tailscale: the daemon runs there as a service, sessions keep running with the laptop closed, one-click setup brings the server your tools, agents and skills. Desktop apps built there open on your screen.
- Copy a project and its sessions to another host, and sync them back both ways.
- Built-in browser for dev servers, ports forwarded, a files panel, memory and CPU of every session, and the `enowx-app` command line.
