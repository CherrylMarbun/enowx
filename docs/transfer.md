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

## Copies that remember their source

A copy made by **Copy to another host…**, or by an agent with `project copy P --to HOST`, is linked to its source (`core/projectlinks`). A project is known by an ID kept in its folder, `.enowx/local/project.json` (the folder ignores itself in git, and syncs and copies leave it out); a copy takes its source's, so the link holds when a folder is renamed or moved, and `project.find` finds a project by it. A link names each copy by its host's daemon (`DaemonInfo.ID`, made once per daemon folder), the host's name as last seen, and the folder. Links live with the user's data, in `settings/project-links.json` under each daemon's folder, which the sync of the user's data takes to every host; what each sync did (when, what went, conflicts) and what both copies had alike after it (the base) stay on the two hosts linked, so a sync run from any device starts from the last one. A sync run by hand (**Sync with another host…**) keeps the link and its base on both hosts too; what this device's `projectsync.json` held is still read for pairs synced before.

A project's menu has **Sync & push on its own…**: where the project came from and where it was copied, each copy's sync, its conflicts, its auto pushes and what syncs and pushes did. **Make HOST the source** turns a link around; **Unlink** forgets it, both copies staying.

### Syncing on its own

A link turned on syncs its copies by itself, at its interval (5 minutes unless set, 5 seconds at least) or as files change: then both hosts watch the folder (`project.watch`, looking it over every two seconds, sizes and times only) and tell the app once it settles (`project.files.changed`), with a sync every 10 minutes anyway. The hosts' daemons can't reach each other: the app syncs, over the connection it shows and those it keeps to the other hosts in the background, while it reaches both hosts. It waits while either is offline, and, when the user says, while a host's CPU is over 85% or this computer runs on battery.

A sync on its own carries the project's files only, never branches or sessions, by the link's rules:

- **Direction**: both ways; or one way, source to copy (the source's version wins, the copy's new files stay) or the reverse.
- **Files**: what git doesn't ignore, or everything with **Follow .gitignore** off (dependencies' folders, `node_modules`, `target`, `dist`, `.venv`…, never); only those matching **include** patterns when any, none matching **exclude** ones (`*.log`, `build`, `src/**/*.test.ts`: a name alone matches anywhere, a folder takes what is inside); `.env` files only when asked.
- **Deleted files** are deleted on the other side too, or never.
- **Large files** (50 MB unless set) are skipped, or always the source's; so are binaries (images, archives, fonts…) changed on both sides with the latter.
- **Bandwidth**: at most so many KB a second.

Only what changed is carried: each file is known by its SHA-256, remembered while its size and time stay.

**Conflicts.** A file changed on both copies since their last sync, or deleted on one and changed on the other, is never overwritten: each copy keeps its version, the dialog and `project conflicts` list it, and the app tells the user (a notification). They compare both versions line by line, and keep one copy's, or both: the other's kept beside it as `name.conflict-<host>.ext`.

## Auto pushes

A project's work can be pushed to its remote on its own, in case it isn't (`core/autopush`): at an interval (30 minutes unless set, a minute at least), or now (**Push now**, `project autopush P --now`), to a branch of its own, `enowx/autosync` unless set. Never `main`, `master` or the branch checked out, and nothing else is pushed; only that branch is ever forced.

The user's branch, staged files and working tree never change: the snapshot is made in an index of its own (`GIT_INDEX_FILE`) from the commit checked out and `git add -A` (what `.gitignore` ignores left out), then `write-tree` and `commit-tree` onto the branch, which `update-ref` moves. Its parents are the branch's last snapshot and the commit checked out when the branch doesn't hold it yet, so a pull request of the branch shows the work on top of the user's. Nothing changed since the last snapshot, nothing is committed or pushed. The commit says `autosync: 12 files · Mac · 2026-10-26 14:30`.

Secrets never go: files of secrets by their names (`.env`, `.env.*` but `.env.example`, keys and certificates, `id_rsa`, `credentials.json`, `.npmrc`…) are left out of the snapshot, and the files changed are scanned for secrets (private keys, AWS, GitHub, GitLab, Anthropic, OpenAI, Google, Slack, Stripe and Telegram tokens, passwords in connection strings, secrets assigned in code): one found, the push stops and says where, never what. A public repository is warned of (as `gh` tells), as the branch shows half-done work. A pull request of the branch can be opened with `gh` (off unless asked).

Auto pushes run in the daemon of the host holding the project, with its git credentials, no app needed; one host pushes a project (the one it was turned on from, or **Push from this host instead**), so two never push over each other. Their settings live with the user's data (`settings/autopush.json`, synced); how each went stays on the host, and in the history.
