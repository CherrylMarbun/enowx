# Side panel

The panel right of the workspace holds tools about the selected session. The panel button at the right of a session's header, or `⌘⇧D`, shows it; the same button at the right of its top row folds it. That row has a button per tool: clicking one shows it in place, its name lit; files opened from Files follow, each in a tab. The panel keeps its tool and its width (drag its left edge) between launches. Every tool reads through the session's daemon, so it works the same for a session on a server.

## Git

What the session changed against its base, and committing, pushing, opening a pull request, merging and discarding it: see [review.md](review.md).

## Checks

The commands that tell whether the session's work holds up, run in its folder (`core/checks`, `checks.*`): **Run all**, or one from its row; each shows whether it passed, how long it took, and its output's end when unfolded (the last 256 KB are kept). They run one after the other, with `CI=true` so test runners neither watch nor ask, give up after 15 minutes, and **Stop** ends them with every process they started.

Without settings, the checks come from the project's files: its Makefile's `lint`, `typecheck`, `build` and `test` targets; else `go vet`, `go build` and `go test` for a Go module, the same scripts of `package.json` run with the package manager its lockfile names, `cargo build` and `cargo test`, or `pytest`. **Edit** sets them for the project instead, kept by the daemon in `~/.enowx/checks.json`, so the project's own files stay untouched.

## Files

The tree of the session's folder (`core/files`, `file.list`, `file.read`): folders first, unfolded one at a time, and read again every 4 seconds while the tab shows, since the agent keeps changing them. Files are marked as git sees them: `M` modified, `A` added, `U` untracked, `D` deleted, `R` renamed; a folder holding changes is marked `M`, and what git ignores is dimmed. Clicking a file opens it in a tab of its own, after the tools in the panel's top row, the tree staying as it was: a Markdown file rendered (headings, lists, tables, code blocks, as the chat renders the agent's answers), with **Raw** in its bar for the text; another file's text, numbered and colored as code by its extension (keywords, strings, comments, numbers; Markdown and plain text stay plain), its first 1 MB; an image (PNG, JPEG, GIF, WebP, BMP or SVG, up to 8 MB) shows as a picture with its size, scaled down to the panel; another binary file says so. It reads again every 4 seconds, the daemon sending nothing when the file hasn't changed since, with its path to copy and a button to open it in the editor. A file open already is shown, not opened twice; `×` closes its tab. Tabs belong to their session, since the panel follows the selected one; the files Activity lists open the same way. The Files tool stays in the session's folder.

A path a session's program prints opens here too, with `⌘`+click on it in the terminal (Ctrl+click elsewhere): relative to the session's folder, from `~/`, or absolute anywhere on the host (`file.read` with `abs`), such as a screenshot an agent saved in `/tmp`; a `:line:col` after it is dropped. A `http://localhost:<port>` link opens in the Browser tool as one of the session's ports; other URLs open in the system browser.

## Dropping files on a server's session

Files dropped on a terminal paste their paths, which an agent takes as files to read (an image to look at, say). On this machine that is all. On a server those paths would name nothing, so the app uploads the files first (`file.upload`, in 4 MB parts, through the daemon's connection) and pastes their paths on the server instead. They go to `~/.enowx/uploads/<session>/` there, out of the session's folder so its git status stays clean, a name taken being numbered (`shot-2.png`), and they are deleted with the session. The app does so for any paste that is only paths of files on this machine, as copying files in Finder and pasting them gives, up to 200 MB at once; any other paste goes as it is.

## Open in editor

A session's folder opens in VS Code, Cursor or Zed from its menu (right-click, or the header's More) and the command palette, and the file shown in Files opens from its bar in the editor last used. They open by the editor's URL scheme: `vscode://file/<path>`, `cursor://…`, `zed://file/<path>` on this machine; on a server, through the editor's own SSH remote, `vscode://vscode-remote/ssh-remote+<user@host><path>` (Cursor alike) and `zed://ssh/<user@host><path>`, so the files stay on the server. The editor needs its remote set up for that host (Remote-SSH for VS Code and Cursor); the server URLs aren't checked against a server yet.

## Activity

What the agent did, read from the transcript the chat view reads (so Claude Code for now; other agents say they aren't read yet). On top, the totals: turns, tool calls, edits and failed calls. Then the files the agent edited, with the lines each edit added and removed summed per file; clicking one shows it in Files. Then the turns, newest first: the prompt, when, how long the agent worked, and what it ran; unfolding a turn lists its tool calls with what each worked on, failed ones in red.

## Ports

The TCP ports the session's processes listen on (`session.ports`): a dev server the agent started, say, with the program and its PID, and whether only the host reaches it or every address. On this machine, **Open** opens it in the browser. On a server, **Forward** listens on this machine's loopback, on the same port when it is free, and carries each connection to the server's port through the SSH connection (`direct-tcpip`, `transport.Forward`); **Open** then opens the local address. Forwards last until stopped, until their session ends, or until the app disconnects from the server; a dev server restarting keeps its forward, connections failing while it is down.

When a session opens a port while the app is connected, a card at the bottom right says so (`node is listening on :5173 in …`) with **Open**, or **Forward & open** for a server, and **Ports** to see the tab; it goes when taken, dismissed or the port closes. The daemon looks every 5 seconds while a client is connected and tells clients what changed (`ports.changed`, with the ports opened); its first look only learns what is open, so ports open before aren't offered.

The daemon finds a session's processes by name: each session's program runs under a small `/bin/sh` named `enowx-session-<id>` (`core/session`), whose arguments every platform shows, where recent macOS hides other processes' environment. Ports come from `lsof` on macOS and `/proc/net/tcp` on Linux; Windows shows none yet.
