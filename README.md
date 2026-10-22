# enowX

A lightweight orchestrator for coding agents. Run Claude Code, Codex, OpenCode, Gemini CLI and more in parallel, each in its own git worktree, on your machine or on your own VPS.

> **Status:** early planning. Nothing to install yet.

- **Local first:** install, open a repo, run agents. No server setup.
- **VPS when you need it:** move the heavy lifting to your own server; your laptop becomes just a screen.
- **One-click VPS setup:** bring your toolchains, agent CLIs, MCP servers, skills and env to a blank VPS in one click.
- **No cloud:** no accounts, no relay, no telemetry by default.

See [docs/PLAN.md](docs/PLAN.md) for the full plan and roadmap, and [CONTRIBUTING.md](CONTRIBUTING.md) to get involved.

## License

[Apache License 2.0](LICENSE)

## Try it

![enowX: projects in the sidebar, sessions in a grid](docs/assets/ui-graphite.png)

Requires Go 1.27+ and git.

```sh
make build
./bin/enowx                          # this machine; open a folder from the sidebar
./bin/enowx --project ~/code/my-app  # also open a folder as a project
./bin/enowx --remote user@host       # a server running ~/.enowx/bin/enowxd
```

Each project is a folder. A new session runs an agent (Claude Code, Codex, OpenCode, Gemini CLI), your shell or any command, either in the project folder or, on git projects, on its own branch in its own worktree.

See [docs/spikes/phase-0.md](docs/spikes/phase-0.md) for the architecture spike results.

## License

[Apache License 2.0](LICENSE)

## Try the spike

Requires Go 1.27+ and git.

```sh
make build
./bin/enowx --repo /path/to/repo --count 4                 # 4 shells in 4 worktrees, locally
./bin/enowx --repo /path/to/repo --agent claude --count 4  # an agent in each worktree
./bin/enowx --remote user@host                             # a server running ~/.enowx/bin/enowxd
```

See [docs/spikes/phase-0.md](docs/spikes/phase-0.md) for results.
