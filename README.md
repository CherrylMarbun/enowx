# enowX

A lightweight orchestrator for coding agents. Run Claude Code, Codex, OpenCode, Gemini CLI and over forty other agents in parallel, each in its own git worktree, on your machine or on your own VPS.

> **Status:** early development, no published release yet. Build it from source, or its installers with `make package` ([docs/install.md](docs/install.md)).

- **Local first:** install, open a repo, run agents. No server setup.
- **VPS when you need it:** move the heavy lifting to your own server; your laptop becomes just a screen.
- **One-click VPS setup:** bring your toolchains, agent CLIs, MCP servers, skills and env to a blank VPS in one click.
- **No cloud:** no accounts, no relay, no telemetry by default.
- **Chat or terminal:** Claude Code and Codex sessions show as a chat too, with edits, tool calls, and messages you can edit and send again.
- **Pipelines:** plan, implement and review a task with agents one after another, on a branch of its own.
- **Scriptable:** the `enowx-app` command line starts, reads, types into and waits for sessions; agents use it to start other agents.

Start with [docs/install.md](docs/install.md). See [docs/PLAN.md](docs/PLAN.md) for the full plan and roadmap, and [CONTRIBUTING.md](CONTRIBUTING.md) to get involved.

## Try it

![enowX: projects in the sidebar, sessions in a grid](docs/assets/ui-graphite.png)

Requires Go 1.27+ and git.

```sh
make build
./bin/enowx                          # this machine; open a folder from the sidebar
./bin/enowx --project ~/code/my-app  # also open a folder as a project
./bin/enowx --remote user@host       # a server running ~/.enowx-app/bin/enowxd
```

Each project is a folder. A new session runs an agent (Claude Code, Codex, OpenCode, Gemini CLI, Kiro, Cursor, Copilot, Amp and [many more](docs/agents.md)), your shell or any command, either in the project folder or, on git projects, on its own branch in its own worktree.

## Develop

```sh
make dev   # hot reload: Go changes rebuild and relaunch the app and its daemon
make test
make lint
```

`make dev` keeps its daemon, sessions and hosts in `~/.enowx-dev` (set `DEV_HOME` to change it), apart from an installed enowX. See [AGENTS.md](AGENTS.md) for the layout and code rules, and [docs/spikes/phase-0.md](docs/spikes/phase-0.md) for the architecture spike.

## License

[MIT License](LICENSE)
