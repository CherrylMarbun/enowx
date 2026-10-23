# Chat view

The chat button in a session's header shows the session as a conversation instead of a terminal. The terminal keeps running behind it: the message box types into it, and approvals, menus and anything the transcript doesn't hold still show in the terminal.

## Where the conversation comes from

The daemon reads the transcript the agent records on its own host (`session.transcript`), following the file as the agent appends to it, so local and remote sessions work the same. Each entry carries a revision; the app polls for the entries changed since the last one it has, which keeps a long conversation cheap to follow.

| Agent | Transcript | Status |
|---|---|---|
| Claude Code | `~/.claude/projects/<folder>/<session>.jsonl`; `<folder>` is the working directory with every character other than a letter or digit replaced by `-`. Records of type `user` and `assistant` carry `message.content`: a string, or blocks `text`, `thinking`, `tool_use` (`id`, `name`, `input`) and `tool_result` (`tool_use_id`, `content`, `is_error`). Records with `isMeta` or `isSidechain` (subagents) aren't the conversation. | Supported. Format as read by `codex-rs/external-agent-migration/src/sessions/records_cla.rs` in openai/codex. |
| Codex | Rollout files under `~/.codex/sessions` | Not yet: the format still needs checking. |
| OpenCode, Gemini CLI, Kiro CLI | — | Not yet. |

## Which file belongs to a session

A session's transcript is the newest file for its folder that changed after the session was created. A new file (after `/clear`, say) takes over; a file another session follows is left to it, so two Claude Code sessions in one folder each keep their own.

## What the chat shows

A long conversation stays light. The chat opens on about the last 300 entries, whole turns (`TranscriptParams.Tail`), and loads 300 more each time the user scrolls up to the oldest (`Before`); daemons before this answer with every entry, which still works. The turns are a virtual list (`ui.List` with `FollowEnd`), which builds only those in view; the poller wakes the app only with news, and every 5 seconds otherwise; Markdown is parsed once per text; and only turns that arrive while the chat shows animate in. The +/− chip counts the entries loaded.

Each message of the user's starts a turn. The agent's thoughts, tool calls and what it said on the way fold into one line, "Worked for 12s · 4 tools · 2 files edited", open while the turn is under way; its answer, the text it ended on, shows under it as Markdown (headings, nested and task lists, quotes, rules, tables, fenced code with a Copy button, inline code, bold, italics, strikethrough, links). An edit opens to its diff; other tools to their output. Over the composer, chips say what the agent is doing and the lines its edits added and removed.

## Changing the agent's settings

The composer shows the agent's model, permission mode and effort, as the transcript and the screen say them, and changes them the way the agent's user would (`core/agent/settings.go`):

| Setting | Claude Code |
|---|---|
| Model | types `/model <name>`, with the names Claude Code records: Default, Opus 5.5, Fable 5.1, Sonnet 5.5, Haiku 5.5, Haiku 4.5, Sonnet 5, Opus 5, Fable 5, Opus 4.8 (the first ten of its `/model` menu on 2.1.308). Read back from `message.model`. |
| Permission mode | the daemon presses Shift+Tab until the screen says the mode (`session.mode`): auto, manual, accept edits, plan, bypass permissions, in that order (checked on 2.1.308). Bypass is in the cycle because enowX starts Claude Code with `--allow-dangerously-skip-permissions`, except as root. Read back from the screen, else from `permissionMode`. |
| Effort | types `/effort <level>`: low, medium, high, xhigh, max. Read back from `effort` on assistant records. |

Other agents show no settings yet.

## Images and files

Pasting an image into the composer (Cmd+V, a screenshot copied, say) saves it as a PNG in `~/.enowx/screenshots` and attaches it to the message, shown as a small preview over it (files copied in Finder show by name), each with an × that drops it; clicking an image shows it large, fitted to the window, with Remove. The message waits for them to upload, then goes as a paste of its text and their paths, which Claude Code takes as images (`[Image #1]`). For a session on a server they are uploaded to its host first (`file.upload`), so the path is one the agent can open there: agents read an image from a path in the prompt. The terminal does the same with an image or files pasted into it; text pastes as text, even with a picture of it on the clipboard (a spreadsheet's cells).

## Slash commands

Typing `/` at the start of the composer opens a menu of the agent's commands: arrows move, Tab or Enter puts the command in the box, Escape closes it. The daemon reads the list once per folder and keeps it for ten minutes (`session.commands`, `core/agent/commands.go`). For Claude Code it is the `slash_commands` of the init record of `claude -p --output-format stream-json --verbose /cost`, run in the session's folder with `--no-session-persistence`, `--strict-mcp-config` and hooks off: `/cost` is answered locally, so nothing reaches a model, no hook runs and no session is saved; MCP prompts are left out. On 2.1.308 it lists 132 commands. Skills and commands on disk (the user's, the project's, the plugins') get their description from their front matter; built-ins have none.

The context meter beside the send button is the tokens of Claude Code's last answer (input, cache reads and writes, output, from `message.usage`) over the model's window. Claude Code doesn't record the window, so it comes from the preset: 1M for Opus 5.5 (its status line shows 141k as 14%) and for models named `[1m]`. For a model whose window hasn't been checked, the meter shows the token count without a percentage.
