# Hooks

A hook is something enowX does when something happens in a session: run a command, give the agent context, or refuse what the agent is about to do. One hook works on every agent. Agents with hooks of their own (Claude Code, Codex) hand each event to enowX before acting on it, so a hook can still refuse it or add context. For the other agents, the daemon sees the event from outside, after the fact.

Settings → **Hooks** lists the host's hooks: each one's switch, Edit and Remove, and **Add a hook**. The dialog has Test, which runs the hook now on a sample event. The **Log** tab shows the latest 200 runs since the daemon started, with what each printed.

## Events

| Event | When | Claude Code, Codex | Other agents |
|---|---|---|---|
| `session.start` | the agent started or was continued | in the agent | from outside |
| `prompt.start` | a prompt is about to reach the agent | in the agent | — |
| `tool.before` | a tool is about to run | in the agent | — |
| `tool.after` | a tool ran (`failed` when it failed) | in the agent | — |
| `permission` | the agent asks for permission | in the agent | — |
| `waiting` | the agent waits for the user | Claude Code | — |
| `turn.done` | the agent finished its turn | in the agent | its screen quiet for 4 s |
| `file.changed` | the files a turn changed (git's view, changed since the turn began) | from outside | from outside |
| `git.commit` | a commit in the session's folder | from outside, looked at every 5 s | from outside |
| `session.end` | its program ended | in the agent | from outside |

`HookSupport` in `core/agent` keeps this table, and the dialog shows it under the event. Hooks are POSIX shell, so on Windows only the events seen from outside run.

## Conditions

Every condition a hook sets must hold. An empty one matches anything.

- **Agents:** base IDs (`claude`, `codex`).
- **Projects:** a project's folder or its folder's name. The session's folder counts too, so a worktree matches its project.
- **Tool or command:** a regular expression, case-insensitive, on the tool's name or its command (`^Bash$`, `git push.*\bmain\b`).
- **Files:** globs from the project's folder, as in project links (`*.go`, `src/**`), on the files changed or touched.
- **Prompt or command text:** a regular expression on the prompt or the command.

## Actions

- **Run a command.** It runs with `/bin/sh` in the session's folder.
  - The event is passed as JSON on stdin (`proto.HookEvent`: `event`, `session`, `agent`, `dir`, `project`, `branch`, `prompt`, `tool`, `command`, `paths`, `commit`, `message`, `via`, `time`).
  - It also gets `ENOWX_HOOK_EVENT`, `ENOWX_HOOK_NAME`, `ENOWX_HOOK_SESSION`, `ENOWX_HOOK_AGENT`, `ENOWX_SESSION` and `ENOWX_BIN`, so it can use enowX's commands.
  - Exiting 2 refuses the action, with stderr as the reason. Exiting 0 with output gives that output to the agent as context, before a prompt or at a session's start.
  - It has a timeout (30 s unless set), and is killed together with what it started.
  - **In the background**, the agent doesn't wait for it.
- **Add context:** text the agent reads before a prompt or at a session's start.
- **Refuse:** the action is refused, and Reason is told to the agent. Only a tool about to run or a prompt can be refused. On other events, the refusal is only logged.

Hooks run in the list's order. The first refusal stops the rest, and contexts add up. On events nothing waits for (seen from outside, or after the fact), every hook runs in the background.

Text from the agent (the prompt, a command) is masked of the vault's credentials lent to the session before a hook sees it.

## How agents hand over events

enowX adds its own hooks when it starts Claude Code (`--settings`) or Codex (`-c hooks.<Event>=…`). They run `"$ENOWX_BIN" hook --event E [--state S]` with the agent's JSON on stdin (`daemon.Hook`). That command calls `session.hook` on the daemon, which sets the state, runs the matching hooks and answers. It then answers the agent as agents read their hooks: exit 2 with the reason on stderr to refuse, or the context on stdout. Outside enowX (no `ENOWX_BIN`), these hooks do nothing.

Codex runs a hook only once the user trusted its exact definition, kept as a hash in `config.toml` (`[hooks.state."<source>:<event>:<group>:<handler>"] trusted_hash`).
- enowX's hooks are the same in every session, so the daemon records them as trusted itself (`agent.TrustCodexHooks`) before starting Codex.
- It adds only those tables, under the source `/<session-flags>/config.toml`, and leaves the rest of the file as it was. A hook the user turned off there stays off.
- The user's own hooks, and a project's `.codex/hooks.json`, still ask in Codex.

## Where hooks are kept

- **The host's own hooks:** `~/.enowx-app/settings/hooks.json`, synced to the user's other hosts with their settings ([user-data-sync.md](user-data-sync.md)). The daemon reads the file again when it changes.
- **A project's hooks:** in its `.enowx/hooks.json`, committed with it, as a list of the same hooks.
  - They don't run until the user allows that file as it is. Settings → Hooks shows the projects waiting, with Review… and Allow them.
  - The approval is of the file's hash, kept per host in `~/.enowx-app/hooks-allowed.json`, so any change asks again.

## Protocol

| Method | What |
|---|---|
| `hooks.list` (ProjectDir) | HooksInfo: the hooks, the project files waiting, each agent's support |
| `hooks.save` ([]Hook) | the host's hooks, all of them |
| `hooks.approve` (HooksApproval) | allow or stop allowing a project's file |
| `hooks.test` (HookTest) | run a hook now on a sample event |
| `hooks.log` | the latest runs |
| `session.hook` (SessionHookParams) | an agent's event: the state, then the hooks' decision |
