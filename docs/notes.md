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

## Asking an agent

**Ask an agent** in the page's header opens a panel beside it to talk with an agent about the page: Claude Code, Codex or any other installed, picked per page. Suggestions start it on the common asks (tidy the page into to-dos, turn it into a plan, brainstorm it as a new project, summarize it), or ask anything.

- The agent works on notes only. It runs in an empty folder of its own (`~/.enowx-app/notes-chat/<note>`), out of the session list, with a guide of its own on writing to-do lists, plans, project ideas, meeting notes and tidy pages. Claude Code is held to it: only enowX's notes tools (list, read, create, write), web search, and none of its tools for files or commands. Other agents are asked to keep to it.
- Nothing it writes takes effect by itself. Each write is a proposal shown above the page, as the lines it changes (a new page in full); **Accept** makes it, **Reject** drops it, and **Undo** puts the page back after accepting. A proposal made before the page changed says so.
- Claude Code and Codex show as a chat; other agents as their terminal. The panel's **+** starts a new conversation.

The daemon keeps proposals in `~/.enowx-app/notes/proposals.json` (`note.proposals`, `note.accept`, `note.reject`, `notes.proposals`). A session started with `NotesFor` sets `ENOWX_NOTES_PROPOSE`, which makes `enowx-app notes write` and `new` propose, refuses `rm` and `move`, and leaves the MCP server with the notes tools alone.

## Groups

Groups keep pages together, like folders: a top-level page is in one group or none, and the pages under it go with it. **New group** (the folder button beside **New**) makes one, with a name, an emoji and a color. In the tree each group is a header with its count of pages: a click folds it (every device sees it folded), **+** makes a page in it, and its menu edits or deletes it. Dragging a page onto a group's header puts it in the group, onto **No group** takes it out, and a page's menu moves it to any group; dragging a group onto another puts it before. Deleting a group keeps its pages, in no group. The folder button beside the filter shows one group's pages alone.

Groups are kept in `~/.enowx-app/notes/groups.json` and a page's group in its header (`group:`); `note.groups`, `note.group.save`, `note.group.delete`, `note.group.move` and `note.setGroup` change them, and `notes.groups` tells the apps.

## The pages

The tree beside the page lists them by group, filtered by title. Dragging a page onto another's upper or lower edge puts it before or after it, onto its middle under it. A page's menu adds a page inside it, moves it to the top level, or deletes it with the pages under it.

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

## Import and export

**Import…** in the menu beside **Notes** (or files dropped on the tree) brings notes in from other apps. The files are this device's: the app sends them to the host (a folder or several files as one .zip) and the host's daemon reads them, so it works the same on a server. What it reads, found out from the files:

| From | What to give it | What becomes of it |
|---|---|---|
| Notion | Its export (Markdown & CSV, or HTML), the .zip as Notion hands it out or unpacked | Pages nested as they were; a database a table, its rows with more than their properties pages under it (or every row a page, when asked); callouts quotes; links between pages `[[links]]`; images kept |
| Notion's API | An integration's token (notion.so/my-integrations), its pages shared with it | Every page and database it sees, the same way; the token goes to Notion for this import only and isn't kept |
| Obsidian, folders of Markdown | The vault or folder (.zip or not) | A folder at the top a group, a folder deeper a page holding its notes (the note of its name beside it, as Obsidian's folder notes); front matter's title, dates and tags; `[[wikilinks]]`, aliases, headings and embedded images kept |
| Markdown, text | A .md or .txt | A note |
| Evernote | An .enex (a notebook) | A note each in a group of the notebook's name, with to-dos, tags and attachments |
| Google Keep | Google Takeout's Keep folder | A note each, checklists to-dos, labels as tags, pictures kept; what is in Keep's trash skipped |
| HTML, Apple Notes | .html files (Apple Notes exports them through other apps) | A note each, a folder at the top a group |
| enowX | An export's .zip (folders) or a backup's .json | The notes as they were: IDs, groups with their icons and colors, order, times, files |

The dialog first shows what it would make (the notes, where they go, titles there already) and then imports them with their progress, which **Stop** takes back; a report says what was skipped and what had no form in enowX (an embed, an equation), kept as a link or text, or left out. HTML is read as data: nothing in it runs, and links keep only web and mail addresses. **Into the group** puts every note at the top in one group; otherwise each goes in the group its folder says.

Importing the same source again finds the notes it made before (Notion's page IDs, the files' paths in a vault, …, kept in `notes/imports.json`): it skips them, writes them again, or makes copies, as picked.

**Export…** in a page's menu (with the pages under it), a group's menu, or **Export all…** writes them on the host and saves the file on this device:

- **Markdown**: one note as a .md with a header (title, icon, group, dates, its ID); several as a .zip of them.
- **Folders**: a .zip of folders of Markdown, a group's notes in its folder and a page's under it in one of its name, their images in `assets/`. Obsidian opens it as a vault and Notion imports it; **Import** makes the same notes again from it.
- **HTML**: one page with contents, its images in it.
- **PDF**: a page per note, its images in it (in the standard fonts, so what they lack, emoji, is left out).
- **Backup**: everything in one .json, images included, which **Import** restores on any host.

A note's images and files live beside the notes in `notes/assets/<its ID>/`, its body naming them `assets/<ID>/<file>`.

The daemon's `note.import` (a job: `note.import.status`, `note.import.cancel`, `note.imports`, events `notes.import`) reads a file or folder of the host; `note.stage` puts the app's files there first. `note.export` writes an export where it's asked, or where `note.export.bytes` reads it for the app.

## Agents

Agents read and write the same notes with `enowx-app notes` ([cli.md](cli.md)) or the MCP server's `notes_list`, `notes_read`, `notes_create`, `notes_write` and `notes_delete`, as when the user asks one to keep a plan or a checklist there; `notes import`, `notes export` and `notes import-status` (MCP `notes_import`, `notes_export`, `notes_import_status`) bring notes in and take them out. The Notes page's own agent, which only proposes changes, can't import or export.
