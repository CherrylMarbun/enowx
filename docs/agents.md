# Agent presets

How enowX starts each agent (`core/agent`). Sessions start with permission prompts skipped when **Skip permission prompts** is on in the New session dialog (the default), and keep that setting when the daemon restarts them with **continue**.

| Agent | Command | First prompt | Skip permission prompts | Continue last conversation | Source |
|---|---|---|---|---|---|
| Claude Code | `claude` | last argument | `--dangerously-skip-permissions` | `--continue` (per directory) | [CLI reference](https://code.claude.com/docs/en/cli-reference), [permission modes](https://code.claude.com/docs/en/permission-modes) |
| Codex | `codex` | last argument | `--dangerously-bypass-approvals-and-sandbox` (alias `--yolo`) | `resume --last` (filtered to the working directory) | `codex-rs/utils/cli/src/shared_options.rs`, `codex-rs/cli/src/main.rs` in openai/codex |
| OpenCode | `opencode` | `--prompt` | `--auto` (explicit `deny` rules still apply) | `--continue` | `packages/web/src/content/docs/{cli,permissions}.mdx` in sst/opencode |
| Gemini CLI | `gemini` | `--prompt-interactive` | `--yolo` | `--resume latest` (per project root) | `docs/reference/configuration.md`, `docs/cli/session-management.md` in the npm package |
| Kiro CLI | `kiro-cli chat` | last argument | `--trust-all-tools` | `--resume` (per directory) | **Unverified**: Kiro's docs were unreachable; flags are those of its predecessor's open-source `chat` command |

## Installing

When an agent is missing, the New session dialog offers to install it: enowX shows the command, then runs it in a session on that host, as the user enowX runs as, so its output and questions stay visible. The daemon reads the login shell's PATH again when the agent list is asked for, so the new agent shows up without a restart.

| Agent | Install command | Source |
|---|---|---|
| Claude Code | `curl -fsSL https://claude.ai/install.sh \| bash` | README of anthropics/claude-code |
| Codex | `curl -fsSL https://chatgpt.com/codex/install.sh \| sh` | README of openai/codex |
| OpenCode | `curl -fsSL https://opencode.ai/install \| bash` | `docs/index.mdx` in sst/opencode |
| Gemini CLI | `npm install -g --prefix "$HOME/.local" @google/gemini-cli` | README of google-gemini/gemini-cli (`npm install -g`); the prefix avoids needing root |
| Kiro CLI | `curl -fsSL https://cli.kiro.dev/install \| bash` | Kiro's CLI install page, as given by the user (unreachable from enowX's build machine) |

## Signing in

Agents run inside a small `/bin/sh` wrapper (PowerShell on Windows) that starts the login shell once the agent exits, so quitting it with Ctrl+C, or a failed sign-in, leaves a prompt in the same folder. The New session dialog's **Sign in** runs the command below in the project folder, which prints a link and a code to open on any device; this is what works on a server without a browser.

| Agent | Sign-in command | Source |
|---|---|---|
| Claude Code | `claude auth login` | Claude Code docs (`claude auth status`), error guides; prints a URL and asks for the code |
| Codex | `codex login --device-auth` | Codex auth docs; a ChatGPT workspace admin must allow device codes |
| OpenCode | `opencode auth login` | OpenCode CLI docs; most providers take an API key |
| Gemini CLI | `NO_BROWSER=true gemini` | Gemini CLI authentication docs; it has no login command, its first screen signs in |
| Kiro CLI | `kiro-cli login --use-device-flow` | Kiro CLI's own error message; Builder ID and IAM Identity Center only, Google and GitHub sign-in are refused remotely |

## Status

The session list shows each agent as working, needing input, done with its turn, or idle. Agents with hooks report it themselves: enowX adds its hooks when it starts them, and they run `"$ENOWX_BIN" hook <state>`, where the daemon sets `ENOWX_BIN` to its own program in every session, so nothing changes in the user's own configuration and the hooks do nothing outside enowX. The other agents, and these between reports, go by their screen: one that changed within the last 1.5 s is working.

| Agent | How | Reports | Source |
|---|---|---|---|
| Claude Code | `--settings '{"hooks": …}'`, merged with the user's settings | `UserPromptSubmit`, `PostToolUse` → working; `Notification` matching `permission_prompt\|elicitation_dialog` → needs input; `Stop` → done | Claude Code hooks docs; checked on 2.1.309 |
| Codex | `-c notify=[…]`: a `/bin/sh` wrapper that reports done, then runs the user's own `notify` from `config.toml` with Codex's argument | end of each turn → done | Codex `notify` config (`agent-turn-complete`) |
| OpenCode, Gemini CLI, Kiro CLI | — | screen only | |

Typing into an agent that asked something, or pressing Enter after it finished, ends that state; a working agent whose screen stood still for 15 s (interrupted, say) goes back to its screen. Hooks are POSIX shell, so on Windows every agent goes by its screen.

When an agent comes to need input or finishes its turn in a session you aren't looking at, the app shows a desktop notification; clicking it opens the session.

## Guide

Agents enowX starts get a short guide (`agent.Guide`), as a skill built in: dev servers show in the Browser tool by themselves, and a desktop app on a server needs `export DISPLAY=$("$ENOWX_BIN" display)` first, installing Xpra if it is missing (see [preview.md](preview.md)). It adds to the user's own instructions and changes no file of theirs.

| Agent | How |
|---|---|
| Claude Code | `--append-system-prompt` |
| Codex | `-c developer_instructions=…`, after the user's own `developer_instructions` from `config.toml`, which `-c` would replace |
| Others | none: they can be told, or run the command themselves |

## Limits

**Agents at once** (command palette) sets how many agents may work at the same time on a host, kept by its daemon in `~/.enowx/settings.json`. An agent session started past it is created at once, with its worktree, but shows as **queued** and runs nothing; the daemon starts it in that session when an agent finishes its turn (as its hooks or its screen tell), oldest first. An agent just started counts as working for 10 s, so the next one doesn't start before it begins. Shells and custom commands are never queued. A daemon restart forgets the queue: the sessions waiting are gone.

**Set time budget…** (a session's menu, or the palette) sets how long its agent may work, counting only while it works. Past it, the daemon interrupts the agent as its user would (Escape for Claude Code and Codex, Ctrl+C for the others), the session says so, and the app notifies. A new budget lifts it. The work counted starts again when the daemon restarts. Budgets are in time only: what an agent costs isn't known for every agent (see Credits).

## Credits

The daemon reads what each signed-in account has left (`core/credits`, `credits.get`) and the app shows it in the title bar chip, its popover and the Credits page. Tokens are read when needed and sent only to the vendor that issued them; enowX never stores, logs or refreshes them, since refreshing would sign the agent out. A reading is reused for 5 minutes. All three sources are undocumented and may change; a changed answer shows as an error on the card.

| Agent | Login read | Asked |
|---|---|---|
| Claude Code | macOS keychain items `Claude Code-credentials*`, else `~/.claude/.credentials.json` | `GET api.anthropic.com/api/oauth/usage` (`anthropic-beta: oauth-2025-04-20`) |
| Codex | `$CODEX_HOME/auth.json` (`~/.codex`) | `GET chatgpt.com/backend-api/wham/usage` with `ChatGPT-Account-Id` |
| Kiro CLI | its own | `kiro-cli chat --no-interactive /usage`, parsed; account from `kiro-cli whoami --format json` |

## Caveats

- **Claude Code refuses `--dangerously-skip-permissions` as root.** When SSH signs in to a server as root, enowX offers to create a regular user (`enowx` by default) and runs its daemon, and so every session, as that user through `su`. SSH keeps signing in as root. Sign in to each agent once from a Shell session as that user. The choice is saved per host; **Manage hosts → Ask user again** resets it. If you keep root, enowX reports the refusal instead of starting the session.
- **Gemini CLI enables its sandbox with `--yolo`** (a Docker image by default). Without Docker, set `GEMINI_SANDBOX=false` in the server's environment, or turn sandboxing off in `~/.gemini/settings.json`, if that is what you want.
- Skipping permission prompts lets an agent run any command and edit any file the user can. Worktrees isolate files between sessions, not from the rest of the machine.
