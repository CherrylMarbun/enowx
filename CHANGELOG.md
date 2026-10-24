# Changelog

Each version's section is also what the app's update dialog shows.

## 0.1.0

The first release.

- Run Claude Code, Codex and 43 other coding agents in parallel, each on its own branch in its own git worktree; fan out one prompt to several and keep the best; pipelines and automations.
- Watch them as terminals, or Claude Code and Codex as a chat; review each session's changes, comment lines back, commit, push, open a pull request and merge.
- Your own server over SSH or Tailscale: the daemon runs there as a service, sessions keep running with the laptop closed, one-click setup brings the server your tools, agents and skills. Desktop apps built there open on your screen.
- Copy a project and its sessions to another host, and sync them back both ways.
- Built-in browser for dev servers, ports forwarded, a files panel, memory and CPU of every session, and the `enowx-app` command line.
