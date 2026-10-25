# Notes

The rail's **Notes** page keeps pages of notes on the connected host: plans, checklists, what an agent found. Pages nest under one another, as a notebook's, and every app connected to the host shows the same ones as they change.

## Writing

A page has an icon, a title and a body, written a block at a time:

- Each line is a block, shown as it reads; a click opens it as text. Enter starts the next block, Backspace at a block's start joins it to the one before, and Up and Down move between them.
- Markdown typed at a block's start makes it another kind: `# `, `## ` and `### ` headings, `- ` a bulleted list, `1. ` a numbered one, `[] ` a to-do, `> ` a quote; `---` and Enter a divider, ` ``` ` and Enter a code block.
- `/` in an empty block lists the kinds of block, narrowed by what follows it (`/todo`); Enter or a click picks one, and a table among them.
- Tab and Shift+Tab indent a list item; Enter on an empty item ends the list. A to-do's box ticks it.
- Text keeps Markdown's **bold**, *italics*, `code`, ~~strikethrough~~ and links. Lines pasted make blocks.
- Dragging a block moves it before the one it is dropped on.
- `/icon` puts an icon in the text, picked as a page's is: `:icon[lucide:check]`, drawn in the line. `/drawing` makes a code block of language `svg`, shown as the drawing it is (cleaned as icons are).

The page saves itself a moment after a change. A page an agent or another app changes while it isn't being edited here is read again.

## Icons

Pages' and groups' icons are SVG, never emoji, so they look the same on every system and follow the theme. The icon above a page's title opens the picker:

- **Library**: the icons of Lucide, Tabler and Phosphor, kept in the app (no network), searched by name and keyword. Favorites and the icons used lately come first; the libraries shown, the favorites and the recent ones are kept on the host (`~/.enowx-app/notes/icons.json`, `note.iconPrefs`, `note.iconPrefs.save`). An icon takes the text's color, which follows the theme, or one picked.
- **Custom**: the user's own SVG, pasted or from a file, cleaned before it is kept (`core/iconlib`): scripts, event handlers, links out, embedded images and foreign content go, its `viewBox` is set, and its colors become the text's unless kept. 24 KB at most once cleaned.

An icon is stored as `lib:name` (`lucide:rocket`), `#rrggbb` after it for a color, or `svg:<base64>` for the user's own. A page without one shows the default (`lucide:file-text`). Saving an emoji as an icon is refused with the icon to use instead.

Pages and groups of before had emoji: the daemon moves them to the closest SVG icon once as it starts, and a title's leading emoji into its page's icon. An emoji with none close gets the default, marked (`iconReview`) with a dot in the tree until the user picks one. Emoji in a page's text stay.

## Asking an agent

**Ask an agent** in the page's header opens a panel beside it to talk with an agent about the page: Claude Code, Codex or any other installed, picked per page. Suggestions start it on the common asks (tidy the page into to-dos, turn it into a plan, brainstorm it as a new project, summarize it), or ask anything.

- The agent works on notes only. It runs in an empty folder of its own (`~/.enowx-app/notes-chat/<note>`), out of the session list, with a guide of its own on writing to-do lists, plans, project ideas, meeting notes and tidy pages. Claude Code is held to it: only enowX's notes tools (list, read, create, write), web search, and none of its tools for files or commands. Other agents are asked to keep to it.
- Nothing it writes takes effect by itself. Each write is a proposal shown above the page, as the lines it changes (a new page in full); **Accept** makes it, **Reject** drops it, and **Undo** puts the page back after accepting. A proposal made before the page changed says so.
- Claude Code and Codex show as a chat; other agents as their terminal. The panel's **+** starts a new conversation.

The daemon keeps proposals in `~/.enowx-app/notes/proposals.json` (`note.proposals`, `note.accept`, `note.reject`, `notes.proposals`). A session started with `NotesFor` sets `ENOWX_NOTES_PROPOSE`, which makes `enowx-app notes write` and `new` propose, refuses `rm` and `move`, and leaves the MCP server with the notes tools alone.

## Groups

Groups keep pages together, like folders: a top-level page is in one group or none, and the pages under it go with it. **New group** (the folder button beside **New**) makes one, with a name, an icon and a color. In the tree each group is a header with its count of pages: a click folds it (every device sees it folded), **+** makes a page in it, and its menu edits or deletes it. Dragging a page onto a group's header puts it in the group, onto **No group** takes it out, and a page's menu moves it to any group; dragging a group onto another puts it before. Deleting a group keeps its pages, in no group. The folder button beside the filter shows one group's pages alone.

Groups are kept in `~/.enowx-app/notes/groups.json` and a page's group in its header (`group:`); `note.groups`, `note.group.save`, `note.group.delete`, `note.group.move` and `note.setGroup` change them, and `notes.groups` tells the apps.

## The pages

The tree beside the page lists them by group, filtered by title. Dragging a page onto another's upper or lower edge puts it before or after it, onto its middle under it. A page's menu adds a page inside it, moves it to the top level, or deletes it with the pages under it.

## Where they are kept

On the host, in `~/.enowx-app/notes`, a Markdown file each (`<id>.md`) with a header saying its title, icon, the page it is under and its place:

```
---
title: Release plan
icon: lucide:rocket
parent: 3fa94c1d2b7e
order: 0
created: 2026-10-25T14:02:11Z
updated: 2026-10-25T14:10:40Z
---

- [ ] tag the release
```

The daemon lists, reads, saves, moves and deletes them (`note.list`, `note.get`, `note.save`, `note.move`, `note.delete`) and tells every app when they change (`notes.changed`).

## Agents

Agents read and write the same notes with `enowx-app notes` ([cli.md](cli.md)) or the MCP server's `notes_list`, `notes_read`, `notes_create`, `notes_write` and `notes_delete` (icons with `icons_search`), as when the user asks one to keep a plan or a checklist there.
