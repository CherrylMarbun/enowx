# Command line

`enowx` (the desktop app's binary) and `enowxd` (the server daemon's) take the same commands, which talk to the daemon of the host they run on (`cli/`). The app or `enowxd serve` must be running there; the command line doesn't start a daemon itself.

| Command | What it does |
|---|---|
| `ls [--json]` | The sessions on the host: ID, agent, state, project, branch, name. |
| `status [SESSION]` | What a session's program is doing: `working`, `waiting` (asks the user), `done`, `idle`, `queued` or `exited(code)`. Without a session, as `ls`. |
| `spawn [flags] [PROMPT]` | Starts an agent and prints its session's ID. `--agent ID` (default `claude`; `agents` lists them), `--project DIR` (default: the project of the session it runs in, else the current folder, added to the host's projects), `--branch NAME` for a new branch in a worktree of its own (`--base` to start it from), `--on BRANCH` for one that exists, `--name`, `--yolo` to skip permission prompts, `--tab` for a tab of the session it runs in, `--json` to print the session. |
| `send [--no-enter] SESSION TEXT...` | Types a message into the session, as a paste, and sends it (`session.input`). |
| `read SESSION` | Prints what the session's screen shows (`session.screen`). |
| `wait [--timeout 30m] SESSION` | Returns once the session's agent is `done` or `waiting`, or has exited, printing which. An agent without hooks counts as done once its screen is quiet for 3 seconds. |
| `note SESSION [TEXT...]` | Sets what the session is at, shown under it in the app and in `ls` (`session.note`, kept across restarts); no text clears it. Agents are told to say so at milestones. |
| `kill SESSION` | Ends the session. |
| `agents [--all]` | The agents installed on the host (`--all`: every preset). |
| `projects` | The host's projects. |
| `pipeline [--branch NAME] [--agents A,B,C] [--project DIR] [--yolo] TASK` | Starts a pipeline (plan, implement, review) on TASK, on a new branch named after it unless given, with the agents given per stage; prints its ID. See [pipelines.md](pipelines.md). |
| `pipelines` | The host's pipelines, each with its stages' status and sessions. |
| `attach SESSION` | Uses the session's terminal in this one, following its size; Ctrl+] leaves it running. |

`SESSION` is a session's ID, the start of one, its name, or `self`: the session the command runs in (`ENOWX_SESSION`).

## Agents starting agents

Sessions have the binary as `$ENOWX_BIN`, so an agent can start others and follow them. Claude Code and Codex are told so in their guide (`core/agent/guide.go`):

```sh
id=$("$ENOWX_BIN" spawn --agent codex --branch fix/parser "Fix the parser's handling of ...")
"$ENOWX_BIN" wait "$id"
"$ENOWX_BIN" read "$id"
```

The sessions it starts show in the app like any other.
