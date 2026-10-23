# Settings

The rail's **Settings** (or the command palette) has five sections, for the connected host where they are a host's.

- **Agents' library**: the agents' skills, MCP servers, subagents, commands, instructions, settings (permissions and hooks) and Claude Code's plugins, global or the project's own; see [sync.md](sync.md#settings-the-agents-library).
- **New sessions**: what New session starts with, each session still free to differ: the agent; in a git project, a new branch and worktree or the project folder; whether agents skip permission prompts; a time budget for agents (see [agents.md](agents.md#limits)); and how many agents may work at once on the host.
- **Environment**: variables every session on the host gets, agents included, such as API keys and tokens (`sessionEnv.list`, `sessionEnv.set`). The host's daemon keeps them in `~/.enowx/session-env.json`, readable by its user alone, and adds them to sessions started or continued after; the app shows their last four characters. Setting one sends its value to that host, which the page says. `PATH`, `HOME` and enowX's own variables can't be set.
- **App**: the editor files open in, desktop notifications, looking for updates by itself, the version, and the keyboard shortcuts.
- **Diagnostics**: the host's daemon (version, pid, since when, its build), the end of its log (`daemon.log`), and **Restart the daemon** (`daemon.restart`): it stops as an update stops it, and connecting again starts it, its sessions continued.
