# Copying a project to another host

A project's menu (right-click it, or its More button) has **Copy to another host…**: the project and the sessions picked go from the connected host to another, from this machine to a server, a server to this machine, or between servers. The host it comes from keeps everything.

## What goes

- **The project's files.** In a git repository, what git keeps: its tracked files and new ones it doesn't ignore, with `.git` (history and branches; not `.git/worktrees`, which name the source's folders). Outside git, every file but dependency, build and cache folders (`node_modules`, `.venv`, `target`, `dist`, `.next`, …). The target installs and builds them again.
- **`.env` files** only with **Include .env files**, which says they hold secrets: enowX sends no secret to a host without the user saying so.
- **Each session picked**, its tabs with it: its name, agent, branch and options, and its screen. Claude Code's conversation goes too (`~/.claude/projects/…`), its paths rewritten to the new folder, and the agent resumes that very conversation (`claude --resume <id>`). Other agents' conversations stay behind for now: their sessions start a new one, which the result says. A worktree session's worktree is made again on its branch, with its uncommitted work. A custom command shows as it ended, as it may not be safe to run twice.

## How

1. The source's daemon packs an archive (`transfer.pack`, `core/transfer`): a manifest, the project's files, then each session's screen, conversation and worktree files, in `~/.enowx-app/transfers/<id>.tar.gz`.
2. The app connects to the target beside the connected host, as it would connect to it, and carries the archive in 1 MB parts (`transfer.read`, `transfer.write`), the dialog counting them.
3. The target's daemon unpacks it into the project's folder, made whole, by its name, inside the folder given (**Into the folder**, by default the same place under the home folder: `/home/u/code/app` goes to `~/code/app`) (`transfer.unpack`; the folder must be new or empty), adds the project, and starts the sessions again. Both archives are deleted; one left by a copy that stopped goes after a day.

**Open on <host>** connects there and shows the project. Both daemons need this version: an older one says it doesn't know the method.
