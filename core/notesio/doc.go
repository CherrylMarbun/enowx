// Package notesio brings notes in from other apps (Notion, Obsidian and
// folders of Markdown, Evernote, Google Keep, Apple Notes, HTML) and takes
// them out (Markdown, a .zip of folders, HTML, PDF, a JSON backup).
//
// An import reads its source into a Batch of Docs first, which a preview
// shows and Apply writes to a notes.Store. Readers write a Doc's body in
// enowX's Markdown with two kinds of references left to resolve once the
// notes have IDs: links to other notes as [[key#anchor|label]], key being
// another Doc's Key or a note's title, and files as ![alt](asset:name),
// name being one of the Doc's Assets.
package notesio

import (
	"time"

	"github.com/enowx/enowx/proto"
)

// Doc is a note read from a source.
type Doc struct {
	// Key names it within its batch, for its children and for links.
	Key string
	// Source names it for good, so importing it again finds the note made
	// before ("notion:<page id>", "obsidian:<vault>/<path>", …).
	Source string
	// ID is the note's own ID in an enowX backup, kept when it is free.
	ID string
	// Parent is the Key of the Doc it goes under, "" for the top.
	Parent string
	// Group is the group a top Doc goes in ("" for none), as its source's
	// folders say; an import's own group wins.
	Group   string
	Title   string
	Icon    string
	Body    string
	Assets  []Asset
	Created time.Time
	Updated time.Time
}

// Asset is a file a Doc shows or attaches: ![alt](asset:Name) in its body.
type Asset struct {
	Name string
	Data []byte
}

// Batch is what a source holds: its notes, parents before their children,
// and what couldn't be taken.
type Batch struct {
	From        string // proto.Import*
	Docs        []Doc
	Groups      []proto.NoteGroup // a backup's, made as they were
	Skipped     []proto.NoteImportIssue
	Unconverted []proto.NoteImportIssue
}

func (b *Batch) skip(where, what string) {
	b.Skipped = append(b.Skipped, proto.NoteImportIssue{Where: where, What: what})
}

func (b *Batch) unconverted(where, what string) {
	for _, u := range b.Unconverted {
		if u.Where == where && u.What == what {
			return
		}
	}
	b.Unconverted = append(b.Unconverted, proto.NoteImportIssue{Where: where, What: what})
}

// Options change how a source is read.
type Options struct {
	From string // proto.Import*; "" finds out
	// RowsAsNotes makes a Notion database a note per row, under a page
	// listing them, rather than a table.
	RowsAsNotes bool
	// Token is a Notion integration's, for proto.ImportNotionAPI.
	Token string
	// NotionAPI is the Notion API's address, a test's server's in tests.
	NotionAPI string
}
