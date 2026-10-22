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

## Try the spike

Requires Go 1.27+ and git.

```sh
make build
./bin/enowx --repo /path/to/repo --count 4                 # 4 shells in 4 worktrees, locally
./bin/enowx --repo /path/to/repo --agent claude --count 4  # an agent in each worktree
./bin/enowx --remote user@host                             # a server running ~/.enowx/bin/enowxd
```

See [docs/spikes/phase-0.md](docs/spikes/phase-0.md) for results.
