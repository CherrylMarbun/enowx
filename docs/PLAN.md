# enowX: Project Plan

> **A lightweight orchestrator for coding agents. Runs on your machine, and when your machine struggles, moves to a VPS in one click.**

Status: draft v0.2 · License: open source · No cloud services (everything runs on the user's machine and the user's own servers)

---

## 1. Summary

enowX is an open source desktop app for running and orchestrating many coding agent CLIs (Claude Code, Codex, OpenCode, Gemini CLI, etc.) in parallel, each in its own git worktree.

- **Local is the primary mode.** Install, open a repo, run agents. No server setup.
- **VPS is an add-on.** When the local machine is too slow, the same workspace can run on the user's own VPS/server. The laptop becomes just a screen, and the load (terminals, agents, MCP, builds, dev servers) moves to the VPS.
- **One-click setup:** the local environment (toolchains, agent CLIs, MCP, skills, GitHub auth, env) is provisioned to a blank VPS in one click.
- **No cloud:** no accounts, relay servers, or enowX backend. Remote connections go directly over SSH / Tailscale to the user's server.

Technically, local and remote use the same engine: the `enowxd` daemon. In local mode the daemon runs on the user's machine; in remote mode it runs on the VPS. The UI and features are identical.

### Problems it solves
1. Running many agents in parallel by hand (tmux + worktrees) is tedious and hard to monitor.
2. Existing orchestration tools tend to be heavy (Electron-based) and full of rarely used features.
3. Developers with low-spec PCs/laptops lag when running many agents, `node_modules`, builds, and dev servers at once.
4. Setting up a VPS for agent development is painful: install toolchains, log in to agents, copy MCP configs, skills, `.env`, GitHub auth. All manual, and versions often drift from local.

### Target users
- Developers who use 2–10 agent CLIs daily (primary users, local mode).
- Users on low-spec laptops who want to offload work to a VPS.
- Developers who want agents to keep running while their laptop is off.

---

## 2. Product principles

1. **Local-first, remote-ready.** Local must work perfectly with zero configuration. Remote uses the same architecture (`enowxd`), so no feature works in only one mode.
2. **Agent-agnostic.** Any program that runs in a terminal can be an agent. Agent-specific integrations are a bonus.
3. **Every feature must speed up the spawn → monitor → compare → merge loop**, or make remote setup easier. Otherwise it is rejected.
4. **Speed is a feature.** App opens in < 300 ms, typing in a remote terminal feels local, 10 terminals stay at 60 fps.
5. **No cloud, no accounts, no telemetry by default.** User data never passes through enowX servers.
6. **Secure by default.** Secrets are never sent without explicit per-item consent.
7. **Keyboard-first.** Every action is available in the command palette.

### Non-goals
- enowX-operated cloud services, accounts, sync servers, or relays.
- Embedded browser, design mode, computer use.
- A full code editor (an "Open in VS Code / Zed / Cursor" button via Remote-SSH is enough).
- Issue tracker integrations (Linear, Jira).
- Fan-out (one prompt to many agents at once) and a scoreboard ranking them: rarely used; sessions side by side are enough to compare by hand.
- Native mobile app.

---

## 3. Tech stack

| Area | Choice | Why |
|---|---|---|
| Desktop app | **Go + [MyGo](https://github.com/egoist/mygo)**, native UI mode | Small binary, low RAM, GPU-rendered, no webview, no cgo |
| Terminal | MyGo `terminal` plugin (**libghostty-vt**) | Ghostty-class emulator. The `Conn io.ReadWriteCloser` + `ResizeByConn` options are designed for remote sessions |
| Daemon (`enowxd`) | Go, single static binary | Same language as the app, easy to deploy to a VPS |
| Transport | SSH (`golang.org/x/crypto/ssh`), optional Tailscale (`tsnet`) | No third-party servers |
| Local storage | SQLite + OS keychain (MyGo `secrets` API) | Secrets never stored in plain files |
| Daemon storage | SQLite in `~/.enowx/` | |
| VPS toolchains | **mise** | Exact same toolchain versions as local |
| Build & release | `mygo build`, GoReleaser (for `enowxd`), GitHub Actions | |

**MyGo risk mitigation (pre-1.0):** `proto/`, `transport/`, `provision/` and `core/` must not import MyGo, with one exception: `core/session` uses MyGo's headless `terminal` plugin to emulate each session's screen on the daemon side (never the `ui` package). Only `ui/`, `cmd/enowx` and that terminal emulation depend on MyGo. The MyGo version is pinned.

---

## 4. Architecture

### Local mode (default)

```
┌──────────── Laptop ────────────────────────────────────┐
│ enowX app (MyGo native)                                │
│        │ local socket (Unix socket / named pipe)       │
│ enowxd (started automatically by the app)              │
│  └ PTY, worktrees, agents, runner… on this machine     │
└────────────────────────────────────────────────────────┘
```

- Users don't need to know the daemon exists. The app starts it automatically (same binary, `enowx daemon` subcommand).
- Because the daemon owns the PTYs, **sessions survive closing the window and restarting/updating the app**. A "stop all agents on quit" option is available.
- Windows: the daemon uses ConPTY and named pipes. macOS/Linux: PTY and Unix sockets.

### Remote mode (optional)

```
┌──────────── Laptop: enowX (MyGo native) ───────────────┐
│ UI: host manager, sidebar, terminal grid, diff, palette│
│ terminal.Options{Conn: stream, ResizeByConn: true}     │
│ Storage: SQLite (hosts, prefs) + OS keychain (secrets) │
└───────────────┬────────────────────────────────────────┘
                │ SSH / Tailscale
                │ one connection, many channels (RPC + PTY streams)
┌───────────────▼──────────── VPS ───────────────────────┐
│ enowxd (Go static binary)                              │
│  ├ session    PTY + scrollback ring buffer, persistent │
│  ├ project    repos & git worktrees (via git CLI)      │
│  ├ agent      presets, lifecycle, status detection     │
│  ├ runner     test/lint/build per worktree             │
│  ├ scheduler  concurrency limits, queue, budgets       │
│  ├ ports      dev server port detection                │
│  ├ provision  manifest execution (check/apply)         │
│  └ store      SQLite                                   │
│ agent CLIs · MCP servers · toolchains (mise)           │
└────────────────────────────────────────────────────────┘
```

### Connection flow
1. The app opens an SSH connection to the host (direct / jump host / Tailscale).
2. The app checks for `enowxd` on the VPS. If missing or outdated, it uploads the matching binary (`linux/amd64` or `linux/arm64`) and runs it as a user service (`systemd --user`, falling back to `nohup`).
3. The app opens an SSH channel to the `enowxd` Unix socket (`~/.enowx/enowxd.sock`). **The daemon never opens a public port.**
4. Over that channel: RPC (control) + one PTY stream per terminal (multiplexed).
5. If the connection drops, sessions keep running in `enowxd`. On reconnect, the app receives a screen + scrollback snapshot and resumes streaming.

### Protocol (`proto/`)
- Framing: length-prefixed frames over one stream, with channel IDs.
- Control: simple JSON-RPC (or MessagePack), protocol version negotiated at handshake.
- PTY data: raw binary frames (output), plus input + resize from the client.
- Events: daemon pushes events (agent status, worktree changes, detected ports, host resources).
- **Compatibility:** app and daemon may run different versions. The handshake declares capabilities, and the app upgrades the daemon when needed.

### One app, many hosts
The sidebar groups everything by host: **"This Mac/PC"** (always present), then any VPSes the user adds. Local and remote workspaces can be open side by side in the same window. WSL is treated as its own host.

---

## 5. Features

Sections are ordered by priority: 5.1–5.3 are the local core, 5.4–5.6 are remote add-ons, and 5.7 is the differentiator that works in both modes.

### 5.1 Workspaces & Agents (core, local and remote)
- Projects (repos) per host; worktrees from a new or existing branch. Non-git folders are supported too.
- Agent presets: `claude`, `codex`, `opencode`, `gemini`, custom command (+ args, env).
- Terminal grid: split, focus, maximize, saved layouts.
- **Agent status:** working / waiting for input / done / error. Sourced from official agent hooks (Claude Code hooks, Codex notify, ACP), with output/idle heuristics as a fallback.
- OS notifications when an agent finishes or needs input. Optional: the user's own ntfy/Telegram.

### 5.2 Review & Merge (core)
- Per-worktree diff against the base branch.
- Comments on diff lines, sent back to the agent as a prompt.
- Actions: commit, push + PR (via `gh`), merge, discard worktree.
- "Open in editor" (VS Code / Cursor / Zed, via Remote-SSH for remote hosts).

### 5.3 Command palette & settings (core)
- Every action via palette and shortcuts.
- Agent presets, theme, terminal font, quit behavior.

### 5.4 Host Manager (remote)
- The "This Mac/PC" host is always present and cannot be removed.
- Host list: name, address, user, port, connection method, tags, default host.
- Import from `~/.ssh/config`.
- Connection methods: direct SSH, jump host/bastion, Tailscale (`tsnet` or an existing tailnet install), local/WSL.
- Secrets (passwords, passphrases, tokens) stored in the OS keychain.
- Host status: online/offline, latency, CPU/RAM/disk, number of running agents.
- Export/import the host list to a file (without secrets) instead of cross-device sync. It can live in the user's dotfiles repo.

### 5.5 One-Click VPS Setup (remote)
**Flow:**
1. Add a host (SSH key, or a one-time password after which enowX installs the public key).
2. The **scanner** reads the local environment and shows a "what to bring" checklist.
3. The user picks items and clicks **Setup**. Progress is shown per step. The process is idempotent and safe to rerun.
4. Done. Pick a repo and agents run right away.

**Manifest (`enowx.toml`):** the scan result, editable, committable, and reusable for other hosts.

| Category | Contents | Notes |
|---|---|---|
| System | base packages, git, build-essential, Docker (optional) | Ubuntu/Debian first |
| Toolchains | Node, Bun, Python, Go, Rust, etc. | via mise, same versions as local |
| Git | `user.name`, `user.email`, signing (optional) | |
| GitHub | repo access + `gh` | **Never copies local tokens.** Uses `gh auth login` device flow on the VPS, or SSH agent forwarding |
| Agent CLIs | claude, codex, opencode, gemini, etc. | same versions as local |
| Agent auth | Claude/Codex login, API keys | device flow on the VPS (shown in the app), or API keys from the keychain |
| MCP | `~/.claude.json`, `.mcp.json`, `~/.codex/config.toml`, etc. | rewrites local paths to remote ones. Local-only MCP servers are flagged and skipped |
| Skills/agents | `~/.claude/skills`, `~/.claude/agents`, commands, global `CLAUDE.md`/`AGENTS.md` | file sync |
| ACP | Agent Client Protocol agent configs | |
| Env & secrets | per-project `.env`, global env | chosen per item, written with `600` permissions |
| Dotfiles | aliases, prompt (optional) | opt-in |
| Repos | clone selected projects | cloned directly on the VPS |

**Provisioner design:** each step is a `Step{ Check() (done bool), Apply() error, Describe() }`. A **dry run** shows what will change before anything runs.

**Drift:** the app detects local config changes (e.g. a new MCP server) and offers "sync to host X?".

**Move workspace:** "Move to VPS" on a local workspace pushes the branch, clones/checks it out on the VPS, and continues the agents there.

### 5.6 Remote ergonomics (remote)
- **Automatic port forwarding:** the daemon detects new dev server ports and the app offers to forward them to `localhost`.
- Host resource dashboard + disk/RAM warnings.
- Drag & drop files/images onto a terminal to upload them (the remote path is pasted automatically).

### 5.7 Compare & Control (differentiator, both modes)
- **Checks:** test/lint/build per worktree, with the results in the side panel.
- **Conflict radar:** warns when two active worktrees change the same file/hunk.
- **Budgets:** time/cost limits per agent (pause/kill).
- **Scheduler:** at most N concurrent agents per host, the rest are queued.

### 5.8 Later
- `enowx` CLI (`enowx spawn`, `enowx status`) so agents can spawn other agents and it can be scripted.
- Non-terminal view: a session shown as a conversation (messages, tool calls, diffs) instead of a terminal. Claude Code ships first, read from its transcript (`docs/chat-view.md`); other agents follow as their formats are checked. The terminal stays available.
- Usage and credit tracking: per host, for every agent signed in there, what each one uses and what is left, read from what the agent itself offers: Codex's app server reports account limits and token use; Claude Code, OpenCode and Gemini CLI record tokens per message locally. Agents that expose no quota say so instead of a guess. Nothing leaves the host, and no agent credential is read.
- Pipelines (Plan → Implement → Review) with handoff between worktrees.
- VPS provider API integrations (Hetzner, DigitalOcean, Vultr) using the user's own API key, to create a new VPS from the app. Still no enowX backend.
- More distros (Fedora, Arch, Alpine).

---

## 6. Security model

| Aspect | Decision |
|---|---|
| Transport | Always SSH or WireGuard (Tailscale). No custom protocol exposed to the internet |
| Daemon | Listens only on a user-owned Unix socket (`0600`) or a named pipe with a user ACL (Windows). Never opens a TCP port |
| Local secrets | OS keychain, never files |
| Secrets on the VPS | Written with `600` permissions; their locations are shown to the user |
| Sending secrets | Explicit per-item consent, shown in the dry run |
| GitHub/agent tokens | Prefer re-login via device flow on the VPS over copying tokens |
| Daemon binary | Checksum (and signature) verified before running |
| SSH host keys | known_hosts verification; fingerprint confirmation for new hosts |
| Telemetry | None by default |
| Vulnerability reports | `SECURITY.md` + GitHub private vulnerability reporting |

**Note:** document that users are responsible for complying with each agent provider's ToS regarding account use on servers.

---

## 7. Repository layout

```
enowx/
  cmd/
    enowx/          desktop app (MyGo)
    enowxd/         daemon
  daemon/           daemon server: connections, RPC handlers, frame pumps
  client/           client for the daemon protocol
  core/             session, worktree, agent, runner, scheduler, ports, store
  proto/            frame, rpc, event, versioning                             (no MyGo)
  transport/        local socket, ssh, jump host, tsnet, daemon spawning      (no MyGo)
  provision/
    scan/           local environment scanner
    manifest/       enowx.toml schema
    steps/          system, mise, git, gh, agents, mcp, skills, env, repo
  ui/
    theme/          design tokens
    hosts/ sidebar/ grid/ diff/ palette/ compare/ setup/
  docs/
  scripts/
  .github/workflows/
```

---

## 8. Roadmap

Assumption: built solo or by a small team, assisted by AI agents. Durations are estimates.

### Phase 0: Spike (1–2 weeks)
Goal: prove the stack, including the remote path, even though remote ships later. The daemon architecture must be proven in both modes from the start.
- [x] Minimal `enowxd`: spawn PTY, local socket, stream output, accept input + resize.
- [x] MyGo app: start the local daemon, render terminals via `terminal.Options{Conn, ResizeByConn}`.
- [x] Grid of 4 terminals, each in a different worktree (any agent CLI via `--agent`; `claude` itself still to be checked).
- [x] Close and reopen the app: sessions survive and the screen is restored.
- [x] Test the same path over an SSH channel to a VPS (just enough to prove it works, unpolished).

**Pass if:** runs locally on macOS/Windows/Linux, app RAM is reasonable with 8 terminals, sessions survive an app restart, and typing over SSH still feels local (< 100 ms RTT). **If it fails, re-evaluate the stack.**

Results: [docs/spikes/phase-0.md](spikes/phase-0.md). Linux passes end to end (GUI included); macOS passes the session tests in CI; the GUI on macOS/Windows and sessions on Windows still need a run.

### Phase 1: Local MVP (4 weeks) → `v0.1`
- [x] Protocol v1 (handshake, RPC, events, multiplexing).
- [x] Automatic local daemon (start, upgrade, shutdown).
- [x] Projects & worktrees, agent presets, terminal grid, saved layouts (per project: split, columns, sessions left out).
- [x] Agent status (Claude Code + Codex hooks, heuristic fallback) + OS notifications ([docs/agents.md](agents.md#status)).
- [x] Diff view, commit/push/PR, merge, discard worktree ([docs/review.md](review.md)).
- [x] Command palette + shortcuts.
- [x] Basic design system (`ui/theme`, `ui/widget`).

**Pass if:** the maintainer uses enowX daily (locally) to build enowX itself.

### Phase 2: First public release (2 weeks) → `v0.2`
- [ ] Installers for macOS (signed + notarized), Windows, Linux (deb + AppImage/script).
- [ ] Auto-update (MyGo updater, from GitHub Releases). The app side is done ([docs/updates.md](updates.md)); the signing key and release feed are not set up yet.
- [ ] Landing page on enowx.ai + docs (getting started).
- [ ] Public repo: README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, issue/PR templates.
- [ ] Announce on X, Reddit, agent community Discords, Hacker News (Show HN). Tease "VPS mode coming soon".

### Phase 3: Remote (3–4 weeks) → `v0.3`
- [ ] Host Manager (SQLite + keychain, `~/.ssh/config` import, host status).
- [x] Auto-install/upgrade `enowxd` over SSH.
- [x] Reconnect with screen snapshots.
- [ ] Local and remote workspaces in the same sidebar.
- [x] Automatic port forwarding + "Open in editor" via Remote-SSH ([docs/side-panel.md](side-panel.md)).

### Phase 4: One-Click Setup (3–4 weeks) → `v0.4`
- [ ] Scanner: toolchains, agent CLIs, MCP, skills, git, env.
- [ ] `enowx.toml` manifest + checklist UI.
- [ ] Provisioner: system, mise, git, gh (device flow), agent CLIs + auth, MCP (path rewrite), skills, env, repo clone.
- [ ] Dry run + per-step progress + idempotency.
- [ ] Drift detection + "Move to VPS".
- [ ] Tailscale via `tsnet` + jump hosts.
- [ ] Docs: VPS setup + security model.

**Pass if:** a blank Ubuntu VPS goes to agents running on the user's repo in < 10 minutes, without opening a terminal manually.

### Phase 5: Compare & Control (4 weeks) → `v0.5`
- [x] Review comments → agent prompt ([docs/review.md](review.md#review-comments)).
- [x] Test/lint/build runner per worktree (a Checks tab in the side panel).
- [ ] Conflict radar.
- [ ] Budgets + scheduler.
- [ ] Host resource dashboard + drag & drop files onto remote terminals.

### Phase 6: Toward `v1.0`
- [ ] `enowx` CLI.
- [ ] Pipelines.
- [ ] VPS provider API integrations (user's own keys).
- [ ] Additional distros.
- [ ] Protocol stabilization (guaranteed backward compatibility).

**Estimate:** first public (local) release around month 2, VPS mode around month 3, one-click setup around month 4, `v1.0` around month 6.

---

## 9. Open source

| Topic | Plan |
|---|---|
| License | **Apache-2.0** (patent protection, enterprise-friendly) or **MIT** (simplest, same as MyGo). Recommendation: Apache-2.0 |
| Repo | `github.com/<org>/enowx`, monorepo |
| Contributing | `CONTRIBUTING.md`, `good first issue` labels, DCO sign-off |
| Agent guide | Root `AGENTS.md` (code rules, layout, test commands) so contributors using AI agents stay consistent |
| Quality | `go vet`, `staticcheck`, `golangci-lint`, CI tests on all 3 OSes. File/function size limits to prevent bloat |
| Releases | SemVer, automated changelog, GitHub Releases + checksums + signatures |
| Design decisions | `docs/decisions/` (short ADRs) |
| Community | GitHub Discussions first, Discord once it grows |
| Funding (optional) | GitHub Sponsors / Open Collective. No paywalled features |

---

## 10. Success metrics

- **Technical:** app startup time, remote typing latency, app RAM with 8 terminals, time from blank VPS to ready.
- **Community (no telemetry):** GitHub stars, downloads per release, issues/PRs from outside contributors, discussion activity.
- **Product:** alpha tester feedback (short survey), whether they still use enowX after 2 weeks.

---

## 11. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| MyGo is pre-1.0 / single maintainer | High | Core without MyGo, pinned version, upstream contributions |
| Terminal issues on Windows (client) | Medium | CI testing from phase 0 |
| Inaccurate agent status detection | Medium | Official hooks + ACP; heuristics only as fallback |
| Rewriting MCP configs across OSes is tricky | Medium | Flag non-portable MCP servers; mandatory dry run |
| Secret leakage during provisioning | High | Per-item consent, keychain, `600` permissions, security audit before v1 |
| Agent config format changes (Claude/Codex) | Medium | Per-agent adapters, per-version test fixtures |
| Scope creep | High | Principle #3 + the non-goals list |
| Competition (other agent orchestration tools) | Medium | Focus on lightweight + identical local/VPS UX + one-click setup |
| A local daemon adds complexity compared to a plain app | Medium | Daemon is invisible to users, ships in the same binary, auto-restarts on crash |

---

## 12. Open decisions

1. License: Apache-2.0 or MIT?
2. Control protocol format: JSON-RPC or MessagePack?
3. VPS daemon as a `systemd --user` service or started on demand by the app? Local daemon: keep running when the app closes (default) or stop with it?
4. Initial VPS distros: Ubuntu 22.04 + 24.04 only, or Debian 12 too?
5. Required agents for the MVP: Claude Code + Codex (others via custom command)?
6. GitHub org name and repo location.
