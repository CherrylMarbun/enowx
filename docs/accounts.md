# Agent accounts

An agent can have other accounts on a host besides the one it signs in with on its own: a work and a personal Claude, say. Each is signed in once, then picked per session, with no new sign-in to switch.

Claude Code and Codex can have them; they keep their sign-in in a folder a variable names (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`). Claude Agent Teams uses Claude Code's.

- **Settings → Agent accounts**: Add account names one and opens a session signing it in. Remove forgets it, with its sign-in.
- **New session** picks the account when the agent has others: Default is its own.
- **Command line**: `enowx spawn --account Work …`.

Each account's home is `~/.enowx/accounts/<id>` on the host (`core/accounts`). It links to what is in the agent's own home (`~/.claude`, `~/.codex`): settings, skills, subagents, commands, plugins and conversations are the same whichever account a session uses, so the chat view, history and Settings see them all. Only the sign-in is the account's own (for Claude Code also `.claude.json`, which holds who is signed in, so MCP servers added with `claude mcp add --scope user` are per account). A session keeps its account when continued, after hibernation or a restart.
