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
