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

The page saves itself a moment after a change. A page an agent or another app changes while it isn't being edited here is read again.

## The pages

The tree beside the page lists them, filtered by title. Dragging a page onto another's upper or lower edge puts it before or after it, onto its middle under it. A page's menu adds a page inside it, moves it to the top level, or deletes it with the pages under it.

## Where they are kept

On the host, in `~/.enowx-app/notes`, a Markdown file each (`<id>.md`) with a header saying its title, icon, the page it is under and its place:

```
---
title: Release plan
icon: 🚀
parent: 3fa94c1d2b7e
order: 0
created: 2026-10-25T14:02:11Z
updated: 2026-10-25T14:10:40Z
---

- [ ] tag the release
```

The daemon lists, reads, saves, moves and deletes them (`note.list`, `note.get`, `note.save`, `note.move`, `note.delete`) and tells every app when they change (`notes.changed`).

## Agents

Agents read and write the same notes with `enowx-app notes` ([cli.md](cli.md)) or the MCP server's `notes_list`, `notes_read`, `notes_create`, `notes_write` and `notes_delete`, as when the user asks one to keep a plan or a checklist there.
