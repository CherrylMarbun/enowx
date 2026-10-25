# Command line

`enowx-app` (the desktop app's binary, linked by that name: `enowx` is another app's) and `enowxd` (the server daemon's) take the same commands, which talk to the daemon of the host they run on (`cli/`). The app or `enowxd serve` must be running there; the command line doesn't start a daemon itself.

| Command | What it does |
|---|---|
| `ls [--json]` | The sessions on the host: ID, agent, state, project, branch, name. |
| `status [SESSION]` | What a session's program is doing: `working`, `waiting` (asks the user), `done`, `idle`, `queued` or `exited(code)`. Without a session, as `ls`. |
| `spawn [flags] [PROMPT]` | Starts an agent and prints its session's ID. `--agent ID` (default `claude`; `agents` lists them), `--agents A,B` to give the prompt to several agents, each on `--branch` with its name after it (a fan-out, printing each session's ID), `--project DIR` (default: the project of the session it runs in, else the current folder, added to the host's projects), `--branch NAME` for a new branch in a worktree of its own (`--base` to start it from), `--on BRANCH` for one that exists, `--name`, `--account NAME` for one of the agent's other accounts ([accounts.md](accounts.md)), `--yolo` to skip permission prompts, `--tab` for a tab of the session it runs in, `--json` to print the session. |
| `send [--no-enter] SESSION TEXT...` | Types a message into the session, as a paste, and sends it (`session.input`). |
| `read SESSION` | Prints what the session's screen shows (`session.screen`). |
| `browser [--session S] open URL\|snapshot\|click REF\|fill REF TEXT\|screenshot FILE\|back\|forward\|reload\|eval JS\|capture on\|off` | Drives the session's Browser in the app: loads a page, prints it as text with its controls numbered, clicks or fills one, saves a PNG, goes back, forward or reloads, runs JavaScript in the page and prints its value, sends its previews through Traffic or not ([preview.md](preview.md#driving-it-from-the-command-line)). |
| `ports [--session S] [preview PORT]` | The ports the session listens on (`session.ports`); `preview` shows one in its Browser. |
| `run [--session S] [N\|NAME\|COMMAND]` | The project's commands, as the Run menu lists them (`commands.list`), or runs one in a new tab of the session (`commands.run`); `--save NAME COMMAND` saves one, `--forget N\|NAME` forgets it. |
| `wait SESSION [--timeout 30m]` | Returns once the session's agent is `done` or `waiting`, or has exited, printing which. An agent without hooks counts as done once its screen is quiet for 3 seconds. |
| `note SESSION [TEXT...]` | Sets what the session is at, shown under it in the app and in `ls` (`session.note`, kept across restarts); no text clears it. Agents are told to say so at milestones. |
| `kill SESSION` | Ends the session. |
| `agents [--all]` | The agents installed on the host (`--all`: every preset). |
| `project list \| add PATH \| clone URL \| status ID \| cancel ID` | The host's projects; a folder made one; a repository cloned on the host with its git credentials into `--dir` (default: here), `--name` (default: the repository's), `--branch`, `--depth N`, `--submodules`, as a new project, showing git's progress until it ends (`--no-wait` prints its ID for `status ID --wait`); a clone stopped, its folder removed. A failure says what to do: a key to add, `gh auth login`, a wrong address (`project.clone`, `project.clone.status`, `project.clone.cancel`, `project.clones`). |
| `projects [group PATH GROUP \| move PATH [--before P] \| move-group GROUP [--before P] \| rename-group FROM TO]` | The host's projects, in the user's order, a project's group after a tab; or arranges them as dragging them in the session list does (`project.arrange`, `project.group.move`, `project.group.rename`). |
| `shells [use PATH \| login]` | The host's shells, the one its terminals start with marked `*`; `use` picks it (`login`: the login shell). |
| `notes [show NOTE \| new TITLE \| write NOTE \| move NOTE \| rm NOTE]` | The user's notes on the host, as a tree; a note's Markdown; a new one (`--parent`, `--group`, `--icon lib:name` or `--icon-svg FILE`, `--body TEXT` or `-` for stdin); changed (`--body` replaces, `--append` adds, `--title`, `--icon`, `--icon-svg`); moved (`--parent`, `--top`, `--before`, or `--group G` into a group); deleted with those under it. `notes groups` lists the groups; `notes group new NAME`, `rename FROM TO`, `icon NAME ICON` and `rm NAME` (its notes stay) change them. Icons are SVG, never emoji. NOTE is an ID or a title ([notes.md](notes.md)). |
| `icons [search QUERY [--lib L] [--limit N] \| libs]` | The SVG icons notes and groups take, found by name and keyword: `lib:name` each, best first, with its keywords; needs no daemon. |
| `pipeline [--branch NAME] [--agents A,B,C] [--project DIR] [--yolo] TASK` | Starts a pipeline (plan, implement, review) on TASK, on a new branch named after it unless given, with the agents given per stage; prints its ID. See [pipelines.md](pipelines.md). |
| `pipelines` | The host's pipelines, each with its stages' status and sessions. |
| `attach SESSION` | Uses the session's terminal in this one, following its size; Ctrl+] leaves it running. |
| `traffic COMMAND` | The HTTP traffic the host's interceptor caught, and its rules: see [below](#traffic) and [traffic.md](traffic.md). |
| `mcp` | Serves the commands to an agent as MCP tools, on stdin and stdout, with the guide as its instructions: `traffic_*`, `browser_*`, `android_*` (status, start, run, logcat, screenshot, controls, tap, swipe, type, key, catch, connect), the sessions' (`sessions_list`, `session_spawn`, `session_wait`, `session_read`, `session_send`, `session_note`), `ports_list`, `run_list`, `run_command`, and `enowx`, which runs any command of this table. Screenshots come back as images ([traffic.md](traffic.md#for-agents)). |
| `android setup` | Brings the host a JDK, the Android SDK, an emulator and a virtual phone, the SDK's licenses read and accepted in the terminal ([android.md](android.md#set-up-android)). |
| `android status\|start AVD\|stop SERIAL` | The SDK, virtual phones and devices adb sees; starts a virtual phone; stops an emulator. |
| `android logcat SERIAL [--package P] [--level W] [--follow]` | The device's log, the app's only with `--package`, at a level and above. |
| `android run SERIAL [--command CMD] [--session S]` | Runs the session's app on the device in a new tab of it, as the Android tool's Run app does; `read` shows the tab. |
| `android connect ADDRESS [--pair ADDRESS --code CODE]` | Connects a phone over Wi-Fi (Wireless debugging), pairing it the first time. |
| `android catch [-s SERIAL] [--off]` | Sends the device's HTTP through Traffic, as a source of the session, and has it trust the CA ([android.md](android.md#catching-a-devices-traffic)). |
| `android screenshot [-s SERIAL] FILE` | Saves the device's screen as a PNG (`android.screenshot`). |
| `android controls [-s SERIAL]` | What is on the device's screen: its controls numbered, with their text, id, state and center (uiautomator's view tree, `android.controls`). |
| `android tap\|longpress [-s SERIAL] REF\|X Y`, `swipe X1 Y1 X2 Y2 [--ms N]`, `type [--into REF] TEXT`, `key NAME\|CODE` | Input, by adb's `input` (`android.act`): a control by its number from the last `controls`, or a point; text typed into the field focused (ASCII); a key by name (back, home, recents, enter, delete, power, volume-up…) or code. `-s` names the device when more than one is online. |
| `api [show\|set\|clear\|restore AGENT]` | Points the host's agents at a model API of the user's, their settings backed up first ([agent-api.md](agent-api.md)). |
| `api [show\|set\|clear\|restore AGENT]` | Points the host's agents at a model API of the user's, their settings backed up first ([agent-api.md](agent-api.md)). |
| `guide` | Prints the guide agents are given: what enowX gives them in a session and how to use it ([agents.md](agents.md#what-agents-are-told)). |

`SESSION` is a session's ID, the start of one, its name, or `self`: the session the command runs in (`ENOWX_SESSION`).

## Traffic

`enowx-app traffic` reads and drives the host's interceptor ([traffic.md](traffic.md)). In a session, it keeps to that session's traffic unless `--all`; secrets stay masked. `F` is the filter language ([traffic.md](traffic.md#filtering)).

| Command | What it does |
|---|---|
| `list [--filter F] [--all] [--limit N] [--json]` | Exchanges: number, status, method, host and path, type, size, time, notes. |
| `show ID [--body request\|response] [--json] [--as LANG]` | One exchange: headers, bodies decoded, messages; or one body whole; or its request as code that sends it again (curl, fetch, python, go, httpie, powershell). |
| `watch [--filter F] [--until-match] [--timeout 1m] [--all] [--json]` | Prints exchanges as they finish; with `--until-match`, the first that matches, then exits. |
| `mock [match] [--status N] [--body TEXT\|--body-file F] [--header 'K: V'] [--times N] [--ws-message TEXT]... [--echo]` | Adds a mock to the project's rules, first. Match: `--method GET,POST`, `--host H`, `--path P` (a glob with `*`), `--query k=v`, `--header-match 'K: V'`, `--body-match TEXT`, `--json-match a.b=v`. Every rule also takes `--name`, `--delay MS`, `--throttle BYTES/S`, `--project DIR`. |
| `breakpoint [match] [--stage request\|response\|both]` | Adds a breakpoint. |
| `forward [match] --to ORIGIN [--keep-host]`, `fail [match] --how 503\|close\|reset\|timeout`, `pass [match]` | Adds a rule sending requests to another origin, failing them, or letting them through before later rules. |
| `transform [match] [--req-set 'K: V'] [--req-remove K] [--req-url URL] [--res-set 'K: V'] [--res-remove K] [--res-status N] [--replace FROM=>TO] [--res-json a.b=VALUE]` | Adds a rule changing requests or their responses on the way (flags repeat). |
| `settings`, `settings set [--insecure H,H] [--upstream URL] [--host NAME=ADDR]... [--client-cert 'HOST CERT KEY']... [--max-exchanges N] [--max-mb N] [--notify-errors on\|off]` | The interceptor's settings, or changed: what isn't given stays (`--clear-hosts`, `--clear-client-certs` empty those). |
| `ca [--pem] [--new]`, `pin\|unpin ID...`, `filters [add\|rm F]` | The CA (made anew with `--new`); pins; the filters saved with the project. |
| `rules [--json]`, `rules rm\|on\|off ID`, `rules clear` | The project's rules. |
| `paused`, `resume ID [--drop] [--status N] [--body TEXT]` | Exchanges held at breakpoints; let one go on, changed or dropped. |
| `send [-X METHOD] [-H 'K: V']... [-d BODY] [--direct] URL` | Sends a request from the host, recorded, and shows it. |
| `clear [--filter F]`, `export [--filter F] [--out FILE]`, `import FILE` | Clears; exports HAR (stdout by default); imports HAR. |
| `attach terminal\|session\|docker\|manual [--agent]`, `attach browser NAME`, `attach electron PATH`, `attach android [SERIAL]`, `attach webview`, `attached`, `detach SOURCE` | Catches more of the session's traffic: a terminal, its tabs, containers, a browser, an Electron app (its executable or `.app`), a device (as `android catch`), the Browser tool's previews; lists and detaches. |
| `env [--off]` | The environment for a shell, as `export` lines: `eval "$(enowx-app traffic env)"`. |
| `status` | The interceptor's port, its CA's fingerprint, what it keeps. |

## Agents starting agents

Sessions have the binary as `$ENOWX_BIN`, so an agent can start others and follow them. Every agent is told so in its guide ([agents.md](agents.md#what-agents-are-told)):

```sh
id=$("$ENOWX_BIN" spawn --agent codex --branch fix/parser "Fix the parser's handling of ...")
"$ENOWX_BIN" wait "$id"
"$ENOWX_BIN" read "$id"
```

The sessions it starts show in the app like any other.
