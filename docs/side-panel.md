# Side panel

The panel right of the workspace holds tools about the selected session. The panel button at the right of a session's header, or `⌘⇧D`, shows it; the same button in its tab strip folds it. Each tool opens as a tab: the launchers on the left of the strip, or `+`, open one, and `×` closes it. The panel keeps its tabs and its width (drag its left edge) between launches. Every tool reads through the session's daemon, so it works the same for a session on a server.

## Git

What the session changed against its base, and committing, pushing, opening a pull request, merging and discarding it: see [review.md](review.md).

## Files

The tree of the session's folder (`core/files`, `file.list`, `file.read`): folders first, unfolded one at a time, and read again every 4 seconds while the tab shows, since the agent keeps changing them. Files are marked as git sees them: `M` modified, `A` added, `U` untracked, `D` deleted, `R` renamed; a folder holding changes is marked `M`, and what git ignores is dimmed. Clicking a file shows its text, numbered (its first 1 MB; a binary file says so), with its path to copy. Paths are relative to the session's folder and can't leave it.

## Activity

What the agent did, read from the transcript the chat view reads (so Claude Code for now; other agents say they aren't read yet). On top, the totals: turns, tool calls, edits and failed calls. Then the files the agent edited, with the lines each edit added and removed summed per file; clicking one shows it in Files. Then the turns, newest first: the prompt, when, how long the agent worked, and what it ran; unfolding a turn lists its tool calls with what each worked on, failed ones in red.

## Ports

The TCP ports the session's processes listen on (`session.ports`): a dev server the agent started, say, with the program and its PID, and whether only the host reaches it or every address. On this machine, **Open** opens it in the browser. On a server, **Forward** listens on this machine's loopback, on the same port when it is free, and carries each connection to the server's port through the SSH connection (`direct-tcpip`, `transport.Forward`); **Open** then opens the local address. Forwards last until stopped or until the app disconnects from the server.

The daemon finds a session's processes by name: each session's program runs under a small `/bin/sh` named `enowx-session-<id>` (`core/session`), whose arguments every platform shows, where recent macOS hides other processes' environment. Ports come from `lsof` on macOS and `/proc/net/tcp` on Linux; Windows shows none yet.
