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

## Caveats

- **Claude Code refuses `--dangerously-skip-permissions` as root.** When SSH signs in to a server as root, enowX offers to create a regular user (`enowx` by default) and runs its daemon, and so every session, as that user through `su`. SSH keeps signing in as root. Sign in to each agent once from a Shell session as that user. The choice is saved per host; **Manage hosts → Ask user again** resets it. If you keep root, enowX reports the refusal instead of starting the session.
- **Gemini CLI enables its sandbox with `--yolo`** (a Docker image by default). Without Docker, set `GEMINI_SANDBOX=false` in the server's environment, or turn sandboxing off in `~/.gemini/settings.json`, if that is what you want.
- Skipping permission prompts lets an agent run any command and edit any file the user can. Worktrees isolate files between sessions, not from the rest of the machine.
