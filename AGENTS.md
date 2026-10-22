# AGENTS.md

Guidance for AI agents and humans working in this repository. Read `docs/PLAN.md` for the product plan and roadmap.

## Commands

- Build: `make build` (outputs to `bin/`; also builds the Linux daemons the app embeds)
- Test: `make test` (or `go test ./path/to/pkg`)
- Lint: `make lint` (`go vet` + `staticcheck`)
- Format: `make fmt` (`gofmt`)

Go 1.27+ is required (MyGo needs it). With an older `go`, set `GOTOOLCHAIN=go1.27.3`.

## Layout

| Path | What lives there | May import MyGo? |
|---|---|---|
| `cmd/enowx` | Desktop app entry point (also runs the daemon via `enowx daemon`) | Yes |
| `cmd/enowxd` | Standalone daemon for servers (`serve`, `bridge`) | Via `daemon` only |
| `daemon/` | Daemon server: connections, RPC handlers, frame pumps | Via `core/session` only |
| `client/` | Client for the daemon protocol | Only `plugins/terminal` types for the `Conn` interface |
| `proto/` | Wire protocol: frames and message types | **No** |
| `core/session` | PTY sessions, kept in headless MyGo terminals | `plugins/terminal` only, never `ui` |
| `core/worktree` | Git worktree management | **No** |
| `core/git` | Thin wrappers over the `git` CLI | **No** |
| `core/project` | Project folders a daemon knows, stored in `~/.enowx/projects.json` | **No** |
| `core/agent` | Agent presets (Claude Code, Codex, …) and how they take a first prompt | **No** |
| `core/hosts` | The device's server list (`~/.enowx/hosts.json`); passwords live in the keychain, never here | **No** |
| `core/tailnet` | Devices on the user's tailnet, via the `tailscale` CLI | **No** |
| `core/envsync` | Copies agents' skills, subagents, commands, memory files, settings and MCP servers between homes (see `docs/sync.md`) | **No** |
| `core/folders` | Lists and creates folders, so the app can browse any host's disk when picking a project | **No** |
| `hostconn/` | Connects to a host's daemon: starts or replaces it locally, installs or upgrades it over SSH | **No** |
| `version/` | The release version shared by app, daemon and client | **No** |
| `core/*` (others) | Runner, scheduler, store (later) | **No** |
| `transport/` | Local socket, SSH, daemon spawning | **No** |
| `ui/` | Native UI views and theme | Yes |

Keep MyGo out of packages marked **No**. If MyGo ever has to be replaced, only `ui/`, `cmd/enowx` and the terminal emulation in `core/session` should change.

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
