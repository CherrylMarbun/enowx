# AGENTS.md

Guidance for AI agents and humans working in this repository. Read `docs/PLAN.md` for the product plan and roadmap.

## Commands

- Build: `make build` (outputs to `bin/`; also builds the Linux daemons the app embeds)
- Develop: `make dev` (hot reload: rebuilds and relaunches the app on Go changes and replaces its daemon; state lives in `~/.enowx-dev`, override with `DEV_HOME=...`)
- Test: `make test` (or `go test ./path/to/pkg`)
- Lint: `make lint` (`go vet` + `staticcheck`)
- Installers: `make package` (every platform's, into `dist/<version>`, never published)
- Servers' distributions: `scripts/test-distros.sh` (docker); the daemon as a systemd service: `scripts/test-service.sh`
- Format: `make fmt` (`gofmt`)

Go 1.27+ is required (MyGo needs it). With an older `go`, set `GOTOOLCHAIN=go1.27.3`.

## Layout

| Path | What lives there | May import MyGo? |
|---|---|---|
| `cmd/enowx` | Desktop app entry point (also runs the daemon via `enowx daemon`) | Yes |
| `cmd/enowxd` | Standalone daemon for servers (`serve`, `bridge`) | Via `daemon` only |
| `daemon/` | Daemon server: connections, RPC handlers, frame pumps | Via `core/session` only |
| `cli/` | The command line (`enowx-app ls`, `spawn`, `send`, `wait`, …), through the host's daemon (see `docs/cli.md`) | **No** |
| `client/` | Client for the daemon protocol | Only `plugins/terminal` types for the `Conn` interface |
| `proto/` | Wire protocol: frames and message types | **No** |
| `core/session` | PTY sessions, kept in headless MyGo terminals | `plugins/terminal` only, never `ui` |
| `core/worktree` | Git worktree management | **No** |
| `core/git` | Thin wrappers over the `git` CLI | **No** |
| `core/project` | Project folders a daemon knows, stored in `~/.enowx-app/projects.json` | **No** |
| `core/accounts` | Agents' other accounts on a host, each a home of its own sharing the agent's settings (see `docs/accounts.md`) | **No** |
| `core/agent` | Agent presets (Claude Code, Codex, …) and how they take a first prompt | **No** |
| `core/hosts` | The device's server list (`~/.enowx-app/hosts.json`); passwords live in the keychain, never here | **No** |
| `core/tailnet` | Devices on the user's tailnet, via the `tailscale` CLI | **No** |
| `core/envsync` | Copies agents' skills, subagents, commands, memory files, settings and MCP servers between homes (see `docs/sync.md`) | **No** |
| `core/folders` | Lists and creates folders, so the app can browse any host's disk when picking a project | **No** |
| `core/transcript` | Reads the conversations agents record on disk, for the chat view (see `docs/chat-view.md`) | **No** |
| `core/review` | A session's changes against its base, and commit, push, pull request (`gh`), merge and discard (see `docs/review.md`) | **No** |
| `core/checks` | Finds and runs a session's checks (tests, linters, build) | **No** |
| `core/commands` | A project's commands to run (found in its files, saved, run lately), for the Run menu | **No** |
| `core/setup` | Scans what a host has, keeps what to bring as `enowx.toml`, and plans the steps that set a server up (see `docs/setup.md`) | **No** |
| `core/transfer` | Packs a project and its sessions into an archive, and unpacks it on another host (see `docs/transfer.md`) | **No** |
| `core/projectsync` | Compares two hosts' copies of a project with their last sync, plans and runs what goes each way (see `docs/transfer.md`) | **No** |
| `core/automation` | When automations (an agent's prompt on a schedule) run (see `docs/automations.md`) | **No** |
| `core/userservice` | Runs the server daemon as a systemd user service, with lingering (see `docs/daemon.md`) | **No** |
| `core/vps` | Creates and deletes servers at Hetzner, DigitalOcean and Vultr with the user's token (see `docs/vps.md`) | **No** |
| `core/mcptest` | Runs an MCP server where its agent would and lists its tools, for Settings' Test | **No** |
| `core/display` | Virtual displays for sessions on Linux servers (Xpra), for desktop apps | **No** |
| `core/files` | Lists and reads a session's folder, with git's marks, for the side panel's Files tool | **No** |
| `core/procstat` | Memory and CPU of process trees and of the machine, for the resource usage view | **No** |
| `hostconn/` | Connects to a host's daemon: starts or replaces it locally, installs or upgrades it over SSH | **No** |
| `version/` | The release version shared by app, daemon and client | **No** |
| `internal/testlisten` | A test binary standing in for a dev server a session starts, for tests of ports | **No** |
| `core/*` (others) | Runner, scheduler, store (later) | **No** |
| `transport/` | Local socket, SSH, daemon spawning | **No** |
| `transport/owntailnet` | The app's own device on the user's tailnet (`tsnet`), apart so the server daemon doesn't carry Tailscale | **No** |
| `ui/` | Native UI: the app shell (host rail, session list, workspace with focus and split views, the side panel of tools about the selected session), the rail's pages (Hosts, Sync, Resource usage), the connection flow and host dialogs | Yes |
| `ui/theme`, `ui/icon` | Palette, fonts, themes; the SVG icons | Yes |
| `ui/widget` | Controls every view shares: buttons, dialogs, menus, chips, Markdown | Yes |
| `ui/textfmt` | How values read in the UI: sizes, plurals, paths | **No** |
| `ui/localpicker` | Picks files and folders of this device, for adding them to a session's folder | Yes |
| `ui/chatview`, `ui/activityview`, `ui/appwindow`, `ui/libraryview`, `ui/browserview`, `ui/checkview`, `ui/diffview`, `ui/fileview`, `ui/portview`, `ui/usageview`, `ui/folderpicker`, `ui/syncview` | Self-contained views; they get what they need from `ui` through options and callbacks, never import it | Yes |

Keep MyGo out of packages marked **No**. If MyGo ever has to be replaced, only `ui/`, `cmd/enowx` and the terminal emulation in `core/session` should change.

New daemon RPC methods go in the `daemon/rpc_*.go` file of their domain; `TestEveryMethodHasHandler` fails when a `proto.Method*` has no handler. The protocol only grows: `go test ./proto` fails when a method, event, constant or JSON field it had is gone or changed; record additions with `go test ./proto -run API -update` (see `docs/protocol.md`). New UI controls go in `ui/widget`; a new self-contained view gets its own `ui/<name>view` package.

## Principles

Every change must respect these (see `docs/PLAN.md` §2):

- **Local and remote are the same code path.** Everything goes through the daemon. Never add a feature that works only locally or only remotely.
- **Agent-agnostic.** Any terminal program can be an agent. Agent-specific code lives behind adapters.
- **Cross-platform.** macOS, Linux and Windows. Guard platform code with build tags (`_unix.go`, `_windows.go`), not runtime string checks scattered around.
- **Mixed versions.** App and daemon update independently. Protocol changes must stay backward compatible or bump `proto.Version` with a fallback.
- **No cloud, no telemetry.** Never add network calls to enowX-operated services.
- **Secrets.** Never log secrets. Never send a secret to a remote host without explicit user consent.

## Code style

- Standard Go: `gofmt`, small packages, errors wrapped with context (`fmt.Errorf("worktree: add %s: %w", name, err)`).
- **Reuse before reimplementing.** Look for an existing function or package before writing a new one.
- **No vague names.** No `utils`, `helpers`, `common`, `misc`. Name files and packages after what they contain.
- **Comments:** explain why, not how. One line when possible. Exported identifiers get doc comments.
- **Small files.** Split a file when it passes ~400 lines or holds more than one responsibility.
- **Concurrency:** document which goroutine owns what. Prefer one owner goroutine + channels over shared mutable state; when using a mutex, keep the locked section small and never call out (callbacks, I/O) while holding it.
- **No attribution of other projects.** Don't write "inspired by", "ported from" or "copied from" in code, comments, docs or commits.

## Tests

- Test behavior, not implementation details. Prefer end-to-end tests through the daemon protocol (`daemon/server_test.go`) over mocks.
- Tests that need `libghostty-vt` download it on first run into the user cache (needs network once).
- Don't check in low-value tests (trivial getters, tests that restate the code).

## Commits

- Imperative, concise subject (`Add session resize to protocol`), body explains why when it isn't obvious.
- One logical change per commit.
