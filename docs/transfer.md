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

## Syncing back

A project's menu also has **Sync with another host…**: it brings the project's copy on the connected host and its copy on another host (made by Copy to another host, or cloned there) to the same state, both ways, for working on a server and carrying the work back, or the other way.

**What tells what changed.** Each daemon scans its copy (`projectsync.scan`, `core/projectsync`): every file git doesn't ignore, by a SHA-256 of its content (remembered while a file's size and time stay, so a large project is read once), each branch's commit, and each session with a hash of Claude Code's conversation (its folders left out, which differ between hosts). Sessions are the same on both hosts by when they were made and their agent, which a copy keeps. The app compares both scans with what both had at the last sync, which it keeps on this device (`~/.enowx-app/projectsync.json`): what differs from that on one side changed there, and goes to the other, deletions included. Never synced, nothing is known deleted: a file one side lacks is new to it.

**Changed on both sides** since the last sync, a file or a conversation is a conflict: the dialog lists them, the newer (a longer conversation) picked, and the user picks whose to keep, one by one or all at once.

**What goes.**
- Files, and a worktree session's work, written over the other side's; deleted ones deleted.
- Commits, as a git bundle of what the other side lacks. A branch moves forward only; one committed to on both sides since stays as it is on the connected host, the other's commits beside it as `<branch>-from-<host>`, to merge. A folder whose checked-out branch moved has git's index follow it, so what isn't committed stays as changes.
- Sessions made on one side are made on the other, as Copy to another host makes them. A conversation that went on comes with its screen; a session running on the side taking it stops and starts again on it.
- A session deleted on one side since the last sync isn't made again, nor deleted on the other.

The connected host sends first, then the other (`projectsync.pack`, carried as a transfer is, `projectsync.apply`); both are scanned again after, their base for next time. `.env` files go only with **Include .env files**.
