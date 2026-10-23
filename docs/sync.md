# Environment sync

**Sync environment…** in the host switcher copies your agents' setup between this device and the connected server, in either direction. Nothing syncs on its own: you pick the items each time.

Each daemon scans its own home (`env.scan`), and the app copies the ticked items one by one (`env.export` on one side, `env.import` on the other). Local and remote take the same path.

## What syncs

| Agent | Item | Where | Source |
|---|---|---|---|
| Claude Code | Skills | `~/.claude/skills/<name>/` (with `SKILL.md`) | `codex-rs/external-agent-migration` in openai/codex |
| | Subagents | `~/.claude/agents/*` | same |
| | Slash commands | `~/.claude/commands/*` | same |
| | Memory | `~/.claude/CLAUDE.md` | same |
| | Settings | `~/.claude/settings.json` | same |
| | MCP servers | `mcpServers` in `~/.claude.json` | same (`mcp.rs`) |
| Codex | Memory | `~/.codex/AGENTS.md` | `codex-rs/tui/src/status/helpers.rs` |
| | Subagents | `~/.codex/agents/*` | `codex-rs/external-agent-migration` |
| | Prompts | `~/.codex/prompts/*` | Codex custom prompts |
| | MCP servers | `[mcp_servers.<name>]` in `~/.codex/config.toml` | `codex-rs/external-agent-migration` |
| Shared | Skills | `~/.agents/skills/<name>/` (read by Codex and OpenCode) | same, and OpenCode's `skills.mdx` |
| OpenCode | Memory, skills, agents, commands | `~/.config/opencode/{AGENTS.md,skills,agents,commands}` | `packages/web/src/content/docs` in sst/opencode |
| | MCP servers | `mcp` in `~/.config/opencode/opencode.json(c)` | `mcp-servers.mdx` |
| Gemini CLI | Memory, commands | `~/.gemini/GEMINI.md`, `~/.gemini/commands/*` | Gemini CLI docs |
| | MCP servers | `mcpServers` in `~/.gemini/settings.json` | Gemini CLI docs |
| Git | Identity | `git config --global user.name`, `user.email` | |

Kiro isn't covered until its config locations are confirmed.

## How items are written

- **Files and folders** replace the item on the other side. A folder is written next to the old one and swapped in, so a failed copy leaves the old one whole. Links (a skill kept in a repository) are copied as their content.
- **MCP servers** are set one by one in the config file; every other key stays as it was, in its order. Codex's TOML is edited table by table, so comments survive; OpenCode's JSONC loses its comments, which the sync reports. Changed config files keep the previous version as `<file>.enowx.bak`.
- **Paths under the home folder** move: `/Users/me/bin/x` on a Mac becomes `/home/enowx/bin/x` on the server. Items compare equal across homes for the same reason.
- The import checks that the program an MCP server runs is installed, and says when it isn't.

## Secrets

Items that look like they carry secrets (MCP `env` and headers, URL parameters and arguments named like keys, `env` in settings, key and `.env` files) are listed with what they hold and are never ticked by default. Ticking one is consent to send it to the other host.

Agent logins (Claude Code's keychain entry or `.credentials.json`, Codex's `auth.json`, and the like) are never synced: sign in to each agent once on each host.

## Settings: the agents' library

The rail's **Settings** shows what the agents on the connected host use, from the same places as Sync, by kind: **Skills**, **MCP servers**, **Subagents**, **Commands**, **Instructions** (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`) and **Agent settings**, each by agent (`ui/libraryview`). Picking one shows it: Markdown rendered (a skill's or a subagent's front matter as fields), an MCP server's entry as its config has it, what in it looks secret, and why it may not work elsewhere. From there:

- **Edit** an item of one text file (instructions, a command, settings) and save it to the host (`env.import`).
- **Open in the editor**, on the host (through the editor's remote for a server).
- Claude Code's `settings.json` shows as **Permissions** (the rules it allows, asks about and denies, and the mode sessions start in) and **Hooks** (a command on an event, for the tools a matcher names), edited as lists and saved with the rest of the file as it was (`envsync.ReadClaudeSettings`, `WriteClaudeSettings`); **File** shows it whole. Without one, **Make Claude Code's settings.json** starts it.
- **Delete** it (`env.delete`): a skill's or a command's files, or the server's entry in the agent's config, which is kept as `.enowx.bak` first.
- **Add an MCP server** for Claude Code, Codex, OpenCode or Gemini CLI: a command (typed as in a shell) or a URL, and environment variables; it is written as that agent keeps its servers (`envsync.NewMCP`).
- **Add a skill** from a folder of this device holding its `SKILL.md`, for Claude Code, OpenCode, or Codex and OpenCode alike (`~/.agents/skills`).

With a project shown, **Global / <project>** at the top switches to the project's own items, under its folder, which the agents load there besides the user's: `CLAUDE.md` (or `.claude/CLAUDE.md`), `AGENTS.md`, `GEMINI.md`, `.mcp.json`, `.claude/skills|agents|commands|settings.json`, `.agents/skills`, `.opencode/…`, `opencode.json`, `.codex/config.toml`, `.gemini/…` (`env.project.*`). Adding there writes into the project, where git sees it.

**Sync to another host…** goes to Sync, which copies items between hosts.
